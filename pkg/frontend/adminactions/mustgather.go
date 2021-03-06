package adminactions

// Copyright (c) Microsoft Corporation.
// Licensed under the Apache License 2.0.

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/Azure/go-autorest/autorest/to"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/Azure/ARO-RP/pkg/util/portforward"
	"github.com/Azure/ARO-RP/pkg/util/ready"
	"github.com/Azure/ARO-RP/pkg/util/version"
)

func (a *adminactions) MustGather(ctx context.Context, w http.ResponseWriter) error {

	ns, err := a.k8sClient.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "openshift-must-gather-",
			Labels: map[string]string{
				"openshift.io/run-level": "0",
			},
		},
	})
	if err != nil {
		return err
	}

	defer func() {
		err = a.k8sClient.CoreV1().Namespaces().Delete(ns.Name, nil)
		if err != nil {
			a.log.Error(err)
		}
	}()

	// wait for default service account to exist
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	err = wait.PollImmediateUntil(10*time.Second, func() (bool, error) {
		_, err := a.k8sClient.CoreV1().ServiceAccounts(ns.Name).Get("default", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	}, timeoutCtx.Done())

	if err != nil {
		return err
	}

	crb, err := a.k8sClient.RbacV1().ClusterRoleBindings().Create(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "must-gather-",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: ns.Name,
			},
		},
	})
	if err != nil {
		return err
	}

	defer func() {
		err = a.k8sClient.RbacV1().ClusterRoleBindings().Delete(crb.Name, nil)
		if err != nil {
			a.log.Error(err)
		}
	}()

	pod, err := a.k8sClient.CoreV1().Pods(ns.Name).Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "must-gather",
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "must-gather-output",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{
				{
					Name:  "gather",
					Image: version.InstallStream.MustGather, // TODO(mjudeikis): We should try get y-version image first and default to latest if not found.
					Command: []string{
						"/usr/bin/gather",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "must-gather-output",
							MountPath: "/must-gather",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "copy",
					Image: version.InstallStream.MustGather,
					Command: []string{
						"sleep",
						"infinity",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "must-gather-output",
							MountPath: "/must-gather",
						},
					},
				},
			},
			TerminationGracePeriodSeconds: to.Int64Ptr(0),
			Tolerations: []corev1.Toleration{
				{
					Operator: "Exists",
				},
			},
		},
	})
	if err != nil {
		return err
	}

	a.log.Info("waiting for must-gather pod")
	err = wait.PollImmediateUntil(10*time.Second, ready.CheckPodIsRunning(a.k8sClient.CoreV1().Pods(pod.Namespace), pod.Name), ctx.Done())
	if err != nil {
		return err
	}

	a.log.Info("must-gather pod running")
	rc, err := portforward.ExecStdout(ctx, a.log, a.env, a.oc, pod.Namespace, pod.Name, "copy", []string{"tar", "cz", "/must-gather"})
	if err != nil {
		return err
	}
	defer rc.Close()

	w.Header().Add("Content-Type", "application/gzip")
	w.Header().Add("Content-Disposition", `attachment; filename="must-gather.tgz"`)

	_, err = io.Copy(w, rc)
	return err
}
