/*
 * k8s.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package k8s

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"time"

	"chain4travel.com/camktncr/pkg/version1"
	"github.com/ava-labs/avalanchego/genesis"
	promVersioned "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
)

const FIELD_MANAGER_STRING = "camktncr-test-net-creator"
const DEFAULT_TIMEOUT = 2 * time.Second

func CreateNamespace(ctx context.Context, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig) error {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: k8sConfig.Namespace,
		},
	}

	_, err := clientset.CoreV1().Namespaces().Create(ctx, &namespace, metav1.CreateOptions{})
	if err != nil && !k8sErrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func CreateNetworkConfigMap(ctx context.Context, clientset *kubernetes.Clientset, genesisConfig genesis.UnparsedConfig, k8sConfig version1.K8sConfig) error {

	genesisJson, err := json.Marshal(genesisConfig)
	if err != nil {
		return err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sConfig.K8sPrefix,
			Namespace: k8sConfig.Namespace,
			Labels:    k8sConfig.Labels,
		},
		BinaryData: map[string][]byte{
			"genesis.json": genesisJson,
		},
	}

	_, err = clientset.CoreV1().ConfigMaps(k8sConfig.Namespace).Get(ctx, k8sConfig.K8sPrefix, metav1.GetOptions{})
	if err == nil {
		err := clientset.CoreV1().ConfigMaps(k8sConfig.Namespace).Delete(ctx, k8sConfig.K8sPrefix, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	_, err = clientset.CoreV1().ConfigMaps(k8sConfig.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	return err
}

//go:embed scripts
var scriptsFs embed.FS

func CreateScriptsConfigMap(ctx context.Context, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig) error {

	files, err := fs.Glob(scriptsFs, "scripts/*")
	if err != nil {
		return err
	}

	data := map[string]string{}
	name := fmt.Sprintf("%s-scripts", k8sConfig.K8sPrefix)

	for _, file := range files {

		stripped := path.Base(file)
		raw, err := scriptsFs.ReadFile(file)
		if err != nil {
			return err
		}
		data[stripped] = string(raw)
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k8sConfig.Namespace,
			Labels:    k8sConfig.Labels,
		},
		Data: data,
	}

	_, err = clientset.CoreV1().ConfigMaps(k8sConfig.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		err := clientset.CoreV1().ConfigMaps(k8sConfig.Namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	_, err = clientset.CoreV1().ConfigMaps(k8sConfig.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	return err
}

func CreateStakerSecrets(ctx context.Context, clientset *kubernetes.Clientset, stakers []version1.Staker, k8sConfig version1.K8sConfig) error {

	secretType := corev1.SecretTypeTLS
	kind := "Secret"
	version := "v1"
	typeMeta := &applymetav1.TypeMetaApplyConfiguration{
		Kind:       &kind,
		APIVersion: &version,
	}

	for i, s := range stakers {
		name := fmt.Sprintf("%s-%d", k8sConfig.K8sPrefix, i)
		secret := &applyv1.SecretApplyConfiguration{
			TypeMetaApplyConfiguration: *typeMeta,
			ObjectMetaApplyConfiguration: &applymetav1.ObjectMetaApplyConfiguration{
				Name:      &name,
				Namespace: &k8sConfig.Namespace,
				Labels:    k8sConfig.Labels,
			},
			Data: map[string][]byte{
				string(corev1.TLSCertKey):       s.CertBytes,
				string(corev1.TLSPrivateKeyKey): s.KeyBytes,
			},
			StringData: map[string]string{
				"Node-ID":       s.NodeID.String(),
				"PublicAddress": s.PublicAddress,
				"PrivateKey":    s.PrivateKey,
			},

			Type: &secretType,
		}

		_, err := clientset.CoreV1().Secrets(k8sConfig.Namespace).Apply(ctx, secret, metav1.ApplyOptions{
			Force:        true,
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func CopySecretFromDefaultNamespace(ctx context.Context, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig, secretName string) error {

	secret, err := clientset.CoreV1().Secrets("default").Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	secret.Labels = k8sConfig.Labels
	secret.Namespace = k8sConfig.Namespace
	secret.ResourceVersion = ""
	_, err = clientset.CoreV1().Secrets(k8sConfig.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if k8sErrors.IsAlreadyExists(err) {
		return nil
	}
	return err

}

func CreateRBAC(ctx context.Context, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig) error {

	saName := k8sConfig.PrefixWith("init-container")

	sa := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: k8sConfig.Namespace,
		},
	}

	saClient := clientset.CoreV1().ServiceAccounts(k8sConfig.Namespace)

	_, foundErr := saClient.Get(ctx, saName, metav1.GetOptions{})
	if foundErr == nil {
		_, err := saClient.Update(ctx, &sa, metav1.UpdateOptions{
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	} else {
		_, err := saClient.Create(ctx, &sa, metav1.CreateOptions{
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	}

	roleName := k8sConfig.PrefixWith("secret-reader")
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: k8sConfig.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "watch", "list"},
			},
		},
	}

	roleClient := clientset.RbacV1().Roles(k8sConfig.Namespace)

	_, foundErr = roleClient.Get(ctx, roleName, metav1.GetOptions{})
	if foundErr == nil {
		_, err := roleClient.Update(ctx, &role, metav1.UpdateOptions{
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	} else {

		_, err := roleClient.Create(ctx, &role, metav1.CreateOptions{
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	}

	rbName := k8sConfig.PrefixWith("read-pods")
	rb := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbName,
			Namespace: k8sConfig.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: saName,
			},
		},

		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			APIGroup: "rbac.authorization.k8s.io",
			Name:     roleName,
		},
	}

	rbClient := clientset.RbacV1().RoleBindings(k8sConfig.Namespace)

	_, foundErr = rbClient.Get(ctx, rbName, metav1.GetOptions{})
	if foundErr == nil {
		_, err := rbClient.Update(ctx, &rb, metav1.UpdateOptions{
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	} else {

		_, err := rbClient.Create(ctx, &rb, metav1.CreateOptions{
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	}

	return nil

}

func CreateApiNodes(ctx context.Context, restClient *rest.Config, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig, numberOfNodes int32) error {

	options := stateFullSetOptions{
		K8sConfig:   k8sConfig,
		Type:        "api",
		IsValidator: false,
		IsRoot:      false,
		Replicas:    numberOfNodes,
		Requests:    k8sConfig.Resources.Api,
	}

	return createStatefulSetWithOptions(ctx, restClient, clientset, options)
}

func CreateRootNode(ctx context.Context, restClient *rest.Config, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig) error {

	options := stateFullSetOptions{
		K8sConfig:   k8sConfig,
		Type:        "root",
		IsValidator: true,
		IsRoot:      true,
		Replicas:    1,
		Requests:    k8sConfig.Resources.Validator,
	}

	return createStatefulSetWithOptions(ctx, restClient, clientset, options)
}

func CreateValidators(ctx context.Context, restClient *rest.Config, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig, numberOfNodes int32) error {

	options := stateFullSetOptions{
		K8sConfig:   k8sConfig,
		Type:        "validator",
		IsValidator: true,
		IsRoot:      false,
		Replicas:    numberOfNodes,
		Requests:    k8sConfig.Resources.Validator,
	}

	return createStatefulSetWithOptions(ctx, restClient, clientset, options)
}

func CreateIngress(ctx context.Context, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig, annotations map[string]string) error {
	pathType := networkingv1.PathTypePrefix

	annotations["nginx.ingress.kubernetes.io/rewrite-target"] = "/$2"
	ingressName := k8sConfig.PrefixWith("ingress")
	nginx := "nginx"
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Namespace:   k8sConfig.Namespace,
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &nginx,
			Rules: []networkingv1.IngressRule{
				{
					Host: fmt.Sprintf("%s.%s", k8sConfig.Namespace, k8sConfig.Domain),
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/static(/|$)(.*)",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: k8sConfig.PrefixWith("root"),
											Port: networkingv1.ServiceBackendPort{
												Name: "rpc",
											},
										},
									},
								},
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: k8sConfig.PrefixWith("api"),
											Port: networkingv1.ServiceBackendPort{
												Name: "rpc",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{
						k8sConfig.Domain,
					},
					SecretName: k8sConfig.PrefixWith("tls-secret"),
				},
			},
		},
	}

	ingClient := clientset.NetworkingV1().Ingresses(k8sConfig.Namespace)
	_, foundErr := ingClient.Get(ctx, ingressName, metav1.GetOptions{})
	if foundErr == nil {
		err := ingClient.Delete(ctx, ingressName, *metav1.NewDeleteOptions(0))
		if err != nil {
			return err
		}

	}
	_, err := ingClient.Create(ctx, &ingress, metav1.CreateOptions{
		FieldManager: FIELD_MANAGER_STRING,
	})

	return err

}

func DeleteCluster(ctx context.Context, restClient *rest.Config, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig, keepDisks bool) error {
	selector, err := metav1.LabelSelectorAsSelector(k8sConfig.Selector())
	if err != nil {
		return err
	}

	selectorString := selector.String()

	err = clientset.AppsV1().StatefulSets(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: selectorString,
	})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	for _, sts := range []string{"api", "root", "validator"} {

		err = clientset.CoreV1().Services(k8sConfig.Namespace).Delete(ctx, k8sConfig.PrefixWith(sts), *metav1.NewDeleteOptions(0))
		if err != nil && !k8sErrors.IsNotFound(err) {
			return err
		}
	}

	err = clientset.CoreV1().ConfigMaps(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: selectorString,
	})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	err = clientset.NetworkingV1().Ingresses(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: selectorString,
	})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	err = clientset.CoreV1().Secrets(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: selectorString,
	})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	err = clientset.CoreV1().ServiceAccounts(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: selectorString,
	})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	err = clientset.RbacV1().RoleBindings(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: selectorString,
	})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	err = clientset.RbacV1().Roles(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: selectorString,
	})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	if !keepDisks {

		err = clientset.CoreV1().PersistentVolumeClaims(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
			LabelSelector: selectorString,
		})
		if err != nil && !k8sErrors.IsNotFound(err) {
			return err
		}
	}

	promClientSet, err := promVersioned.NewForConfig(restClient)
	if err != nil {
		return err
	}

	err = promClientSet.MonitoringV1().ServiceMonitors(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: selectorString,
	})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	return nil
}
