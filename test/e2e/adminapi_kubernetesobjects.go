package e2e

// Copyright (c) Microsoft Corporation.
// Licensed under the Apache License 2.0.

import (
	"context"
	"net/http"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/Azure/ARO-RP/pkg/api"
)

var _ = Describe("[Admin API] Kubernetes objects action", func() {
	BeforeEach(skipIfNotInDevelopmentEnv)

	const objName = "e2e-test-object"
	resourceID := resourceIDFromEnv()

	When("in a standard openshift namespace", func() {
		const namespace = "default"

		It("should be able to create, get, list, update and delete objects", func() {
			defer func() {
				// When ran successfully this test should delete the object,
				// but we need to remove the object in case of failure
				// to allow us to run this test against the same cluster multiple times.
				By("deleting the config map via Kubernetes API")
				err := clients.Kubernetes.CoreV1().ConfigMaps(namespace).Delete(objName, &metav1.DeleteOptions{})
				// On successfully we expect NotFound error
				if !errors.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
			}()

			testConfigMapCreateOK(resourceID, objName, namespace)
			testConfigMapGetOK(resourceID, objName, namespace)
			testConfigMapListOK(resourceID, objName, namespace)
			testConfigMapUpdateOK(resourceID, objName, namespace)
			testConfigMapDeleteOK(resourceID, objName, namespace)
		})

		testSecretOperationsForbidden(resourceID, objName, namespace)
	})

	When("in a customer namespace", func() {
		const namespace = "e2e-test-namespace"

		It("should be able to get and list existing objects, but not update and delete or create new objects", func() {
			By("creating a test customer namespace via Kubernetes API")
			_, err := clients.Kubernetes.CoreV1().Namespaces().Create(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				By("deleting the test customer namespace via Kubernetes API")
				err := clients.Kubernetes.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())

				// To avoid flakes, we need it to be completely deleted before we can use it again
				// in a separate run or in a separate It block
				By("waiting for the test customer namespace to be deleted")
				err = wait.PollImmediate(10*time.Second, time.Minute, func() (bool, error) {
					_, err := clients.Kubernetes.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
					if err == nil {
						return false, nil
					}
					if !errors.IsNotFound(err) {
						return false, err
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())
			}()

			testConfigMapCreateOrUpdateForbidden("creating", resourceID, objName, namespace)

			By("creating an object via Kubernetes API")
			_, err = clients.Kubernetes.CoreV1().ConfigMaps(namespace).Create(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: objName},
			})
			Expect(err).NotTo(HaveOccurred())

			testConfigMapGetOK(resourceID, objName, namespace)
			testConfigMapListOK(resourceID, objName, namespace)
			testConfigMapCreateOrUpdateForbidden("updating", resourceID, objName, namespace)
			testConfigMapDeleteForbidden(resourceID, objName, namespace)
		})

		testSecretOperationsForbidden(resourceID, objName, namespace)
	})
})

func testSecretOperationsForbidden(resourceID, objName, namespace string) {
	It("should not be able to create a secret", func() {
		By("creating a new secret via RP admin API")
		obj := mockSecret(objName, namespace)
		var cloudErr api.CloudError
		resp, err := adminRequest(context.Background(), http.MethodPost, "/admin"+resourceID+"/kubernetesobjects", nil, obj, &cloudErr)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		Expect(cloudErr.Code).To(Equal(api.CloudErrorCodeForbidden))
	})

	It("should not be able to get a secret", func() {
		By("requesting a secret via RP admin API")
		params := url.Values{
			"kind":      []string{"secret"},
			"namespace": []string{namespace},
			"name":      []string{objName},
		}
		var cloudErr api.CloudError
		resp, err := adminRequest(context.Background(), http.MethodGet, "/admin"+resourceID+"/kubernetesobjects", params, nil, &cloudErr)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusForbidden))

		By("checking response for an error")
		Expect(cloudErr.Code).To(Equal(api.CloudErrorCodeForbidden))
	})

	It("should not be able to get a list of secrets", func() {
		By("requesting a list of Secret objects via RP admin API")
		params := url.Values{
			"kind":      []string{"secret"},
			"namespace": []string{namespace},
		}
		var cloudErr api.CloudError
		resp, err := adminRequest(context.Background(), http.MethodGet, "/admin"+resourceID+"/kubernetesobjects", params, nil, &cloudErr)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusForbidden))

		By("checking response for an error")
		Expect(cloudErr.Code).To(Equal(api.CloudErrorCodeForbidden))
	})

	It("should not be able to delete a secret", func() {
		By("deleting the secret via RP admin API")
		params := url.Values{
			"kind":      []string{"secret"},
			"namespace": []string{namespace},
			"name":      []string{objName},
		}
		var cloudErr api.CloudError
		resp, err := adminRequest(context.Background(), http.MethodDelete, "/admin"+resourceID+"/kubernetesobjects", params, nil, &cloudErr)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		Expect(cloudErr.Code).To(Equal(api.CloudErrorCodeForbidden))
	})
}

func testConfigMapCreateOK(resourceID, objName, namespace string) {
	By("creating a new object via RP admin API")
	obj := mockConfigMap(objName, namespace)
	resp, err := adminRequest(context.Background(), http.MethodPost, "/admin"+resourceID+"/kubernetesobjects", nil, obj, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	By("checking that the object was created via Kubernetes API")
	cm, err := clients.Kubernetes.CoreV1().ConfigMaps(namespace).Get(objName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(obj.Namespace).To(Equal(cm.Namespace))
	Expect(obj.Name).To(Equal(cm.Name))
	Expect(obj.Data).To(Equal(cm.Data))
}

func testConfigMapGetOK(resourceID, objName, namespace string) {
	By("getting an object via RP admin API")
	params := url.Values{
		"kind":      []string{"configmap"},
		"namespace": []string{namespace},
		"name":      []string{objName},
	}
	var obj corev1.ConfigMap
	resp, err := adminRequest(context.Background(), http.MethodGet, "/admin"+resourceID+"/kubernetesobjects", params, nil, &obj)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	By("comparing it to the actual object retrived via Kubernetes API")
	cm, err := clients.Kubernetes.CoreV1().ConfigMaps(namespace).Get(objName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(obj.Namespace).To(Equal(cm.Namespace))
	Expect(obj.Name).To(Equal(cm.Name))
	Expect(obj.Data).To(Equal(cm.Data))
}

func testConfigMapListOK(resourceID, objName, namespace string) {
	By("requesting a list of objects via RP admin API")
	params := url.Values{
		"kind":      []string{"configmap"},
		"namespace": []string{namespace},
	}
	var obj corev1.ConfigMapList
	resp, err := adminRequest(context.Background(), http.MethodGet, "/admin"+resourceID+"/kubernetesobjects", params, nil, &obj)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	By("comparing names from the list action response with names retrived via Kubernetes API")
	var names []string
	for _, o := range obj.Items {
		names = append(names, o.Name)
	}
	Expect(names).To(ContainElement(objName))
}

func testConfigMapUpdateOK(resourceID, objName, namespace string) {
	By("updating the object via RP admin API")
	obj := mockConfigMap(objName, namespace)
	obj.Data["key"] = "new_value"

	resp, err := adminRequest(context.Background(), http.MethodPost, "/admin"+resourceID+"/kubernetesobjects", nil, obj, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	By("checking that the object changed via Kubernetes API")
	cm, err := clients.Kubernetes.CoreV1().ConfigMaps(namespace).Get(objName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(cm.Namespace).To(Equal(namespace))
	Expect(cm.Name).To(Equal(objName))
	Expect(cm.Data).To(Equal(map[string]string{"key": "new_value"}))
}

func testConfigMapDeleteOK(resourceID, objName, namespace string) {
	By("deleting the object via RP admin API")
	params := url.Values{
		"kind":      []string{"configmap"},
		"namespace": []string{namespace},
		"name":      []string{objName},
	}
	resp, err := adminRequest(context.Background(), http.MethodDelete, "/admin"+resourceID+"/kubernetesobjects", params, nil, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	// To avoid flakes, we need it to be completely deleted before we can use it again
	// in a separate run or in a separate It block
	By("waiting for the configmap to be deleted")
	err = wait.PollImmediate(10*time.Second, time.Minute, func() (bool, error) {
		_, err = clients.Kubernetes.CoreV1().ConfigMaps(namespace).Get(objName, metav1.GetOptions{})
		if err == nil {
			return false, nil
		}
		if !errors.IsNotFound(err) {
			return false, err
		}
		return true, nil
	})
	Expect(err).NotTo(HaveOccurred())
}

func testConfigMapCreateOrUpdateForbidden(operation, resourceID, objName, namespace string) {
	By(operation + " a new object via RP admin API")
	obj := mockConfigMap(objName, namespace)
	var cloudErr api.CloudError
	resp, err := adminRequest(context.Background(), http.MethodPost, "/admin"+resourceID+"/kubernetesobjects", nil, obj, &cloudErr)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
	Expect(cloudErr.Code).To(Equal(api.CloudErrorCodeForbidden))
}

func testConfigMapDeleteForbidden(resourceID, objName, namespace string) {
	By("deleting the object via RP admin API")
	params := url.Values{
		"kind":      []string{"configmap"},
		"namespace": []string{namespace},
		"name":      []string{objName},
	}
	var cloudErr api.CloudError
	resp, err := adminRequest(context.Background(), http.MethodDelete, "/admin"+resourceID+"/kubernetesobjects", params, nil, &cloudErr)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
	Expect(cloudErr.Code).To(Equal(api.CloudErrorCodeForbidden))
}

func mockSecret(name, namespace string) corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func mockConfigMap(name, namespace string) corev1.ConfigMap {
	return corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"key": "value",
		},
	}
}
