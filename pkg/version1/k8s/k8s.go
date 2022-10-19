/*
 * k8s.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package k8s

import (
	"bytes"
	"context"
	"crypto/sha1"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"log"

	"chain4travel.com/camktncr/pkg/version1"
	"github.com/chain4travel/caminogo/genesis"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
)

const FIELD_MANAGER_STRING = "camktncr-test-net-creator"

func RegisterValidators(ctx context.Context, restClient *rest.Config, k8sConfig version1.K8sConfig, stakers []version1.Staker) error {
	roundTripper, upgrader, err := spdy.RoundTripperFor(restClient)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", k8sConfig.Namespace, fmt.Sprintf("%s-root-0", k8sConfig.K8sPrefix))
	hostIP := strings.TrimLeft(restClient.Host, "htps:/")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)

	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	forwarder, err := portforward.New(dialer, []string{"9650"}, stopChan, readyChan, out, errOut)
	if err != nil {
		return err
	}

	go func() {
		for range readyChan { // Kubernetes will close this channel when it has something to tell us.
		}
		if len(errOut.String()) != 0 {
			panic(errOut.String())
		} else if len(out.String()) != 0 {
			fmt.Println(out.String())
		}
	}()

	go func() {
		if err = forwarder.ForwardPorts(); err != nil { // Locks until stopChan is closed.
			fmt.Println(err)
		}
	}()
	time.Sleep(1 * time.Second)
	defer close(stopChan)

	for {
		err := isBootstrapped()
		if err != nil {
			log.Println("root has not bootstrapped yet")
		} else {
			break
		}
	}

	for _, staker := range stakers {
		err = registerValidator(staker)
		if err != nil {
			return err
		}
		time.Sleep(5 * time.Second)
	}

	return nil
}

func registerValidator(staker version1.Staker) error {
	day, err := time.ParseDuration("24h")
	if err != nil {
		return err
	}

	stakeDur := day * 30
	username := staker.PublicAddress
	sum := sha1.Sum([]byte(username))
	password := hex.EncodeToString(sum[:])
	addr := fmt.Sprintf("P-%s", strings.Split(staker.PublicAddress, "-")[1])

	createUserPostData := strings.NewReader(fmt.Sprintf(`{
			"jsonrpc":"2.0",
			"id"     :1,
			"method" :"keystore.createUser",
			"params" :{
				"username": "%s",
				"password": "%s"
			}
		}`, username, password))
	res, err := http.Post("http://localhost:9650/ext/keystore", "application/json", createUserPostData)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	fmt.Println(string(body))

	importKeyPostData := strings.NewReader(fmt.Sprintf(`{
			"jsonrpc":"2.0",
			"id"     :1,
			"method" :"platform.importKey",
			"params" :{
				"username":"%s",
				"password":"%s",
				"privateKey":"%s"
			}
		}`, username, password, staker.PrivateKey))
	res, err = http.Post("http://localhost:9650/ext/bc/P", "application/json", importKeyPostData)
	if err != nil {
		return err
	}

	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	fmt.Println(string(body))

	now := time.Now().Add(2 * time.Minute)
	endTime := now.Add(stakeDur)

	addVaidatorPostData := strings.NewReader(fmt.Sprintf(`{
			"jsonrpc":"2.0",
			"id"     :1,
			"method": "platform.addValidator",
			"params": {
				"nodeID":"NodeID-%s",
				"startTime": %d,
				"endTime": %d,
				"stakeAmount": %d,
				"rewardAddress": "%s",
				"delegationFeeRate": 10,
				"username": "%s",
				"password": "%s"
			}
		}`, staker.NodeID, now.Unix(), endTime.Unix(), staker.Stake, addr, username, password))
	res, err = http.Post("http://localhost:9650/ext/bc/P", "application/json", addVaidatorPostData)
	if err != nil {
		return err
	}

	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	fmt.Println(string(body))

	return nil

}

func isBootstrapped() error {
	url := "http://localhost:9650/ext/info"
	method := "POST"

	payload := strings.NewReader(`{
    "jsonrpc":"2.0",
    "id"     :1,
    "method" :"info.isBootstrapped",
    "params": {
        "chain": "P"
    }
}`)
	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	parsed := make(map[string]interface{})

	json.Unmarshal(body, &parsed)

	result := parsed["result"].(map[string]interface{})

	if result["isBootstrapped"] != true {
		return fmt.Errorf("not bootstrapped yet")
	}

	return nil
}

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

func CopyPullSecret(ctx context.Context, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig) error {

	secret, err := clientset.CoreV1().Secrets("kopernikus").Get(ctx, "gcr-image-pull", metav1.GetOptions{})
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

func CreateApiNodes(ctx context.Context, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig, numberOfNodes int32) error {

	options := stateFullSetOptions{
		K8sConfig:   k8sConfig,
		Type:        "api",
		IsValidator: false,
		IsRoot:      false,
		Replicas:    numberOfNodes,
		Requests:    k8sConfig.Resources.Api,
	}

	return createStatefulSetWithOptions(ctx, clientset, options)
}

func CreateRootNode(ctx context.Context, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig) error {

	options := stateFullSetOptions{
		K8sConfig:   k8sConfig,
		Type:        "root",
		IsValidator: true,
		IsRoot:      true,
		Replicas:    1,
		Requests:    k8sConfig.Resources.Validator,
	}

	return createStatefulSetWithOptions(ctx, clientset, options)
}

func CreateValidators(ctx context.Context, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig, numberOfNodes int32) error {

	options := stateFullSetOptions{
		K8sConfig:   k8sConfig,
		Type:        "validator",
		IsValidator: true,
		IsRoot:      false,
		Replicas:    numberOfNodes,
		Requests:    k8sConfig.Resources.Validator,
	}

	return createStatefulSetWithOptions(ctx, clientset, options)
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
					Host: k8sConfig.Domain,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     fmt.Sprintf("/%s(/|$)(.*)", k8sConfig.K8sPrefix),
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
								{
									Path:     fmt.Sprintf("/%s/static(/|$)(.*)", k8sConfig.K8sPrefix),
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
					SecretName: fmt.Sprintf("%s-%s-ingress-tls", k8sConfig.K8sPrefix, k8sConfig.Domain),
				},
			},
		},
	}

	ingClient := clientset.NetworkingV1().Ingresses(k8sConfig.Namespace)
	_, foundErr := ingClient.Get(ctx, ingressName, metav1.GetOptions{})
	if foundErr == nil {
		_, err := ingClient.Update(ctx, &ingress, metav1.UpdateOptions{
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	} else { //TODO CHECK FOR ACTUAL NOT FOUND ERROR
		_, err := ingClient.Create(ctx, &ingress, metav1.CreateOptions{
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func DeleteCluster(ctx context.Context, clientset *kubernetes.Clientset, k8sConfig version1.K8sConfig, keepDisks bool) error {

	sel, err := k8sConfig.Selector()
	if err != nil {
		return err
	}

	err = clientset.AppsV1().StatefulSets(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: sel,
	})
	if err != nil {
		return err
	}

	for _, sts := range []string{"api", "root", "validator"} {

		err = clientset.CoreV1().Services(k8sConfig.Namespace).Delete(ctx, k8sConfig.PrefixWith(sts), *metav1.NewDeleteOptions(0))
		if err != nil {
			return err
		}
	}

	err = clientset.CoreV1().ConfigMaps(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: sel,
	})
	if err != nil {
		return err
	}
	err = clientset.CoreV1().Secrets(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: sel,
	})
	if err != nil {
		return err
	}
	err = clientset.CoreV1().ServiceAccounts(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: sel,
	})
	if err != nil {
		return err
	}

	err = clientset.RbacV1().RoleBindings(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: sel,
	})
	if err != nil {
		return err
	}
	err = clientset.RbacV1().Roles(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
		LabelSelector: sel,
	})
	if err != nil {
		return err
	}

	if !keepDisks {

		err = clientset.CoreV1().PersistentVolumeClaims(k8sConfig.Namespace).DeleteCollection(ctx, *metav1.NewDeleteOptions(0), metav1.ListOptions{
			LabelSelector: sel,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
