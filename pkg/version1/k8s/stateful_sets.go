/*
 * stateful_sets.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package k8s

import (
	"context"
	"fmt"
	"strconv"

	"chain4travel.com/camktncr/pkg/version1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

func buildService(options stateFullSetOptions) corev1.Service {

	servicePorts := []corev1.ServicePort{
		{Name: "rpc", Port: 9650, TargetPort: intstr.FromInt(9650)},
	}
	if options.IsValidator {
		servicePorts = append(servicePorts,
			corev1.ServicePort{Name: "staking", Port: 9651, TargetPort: intstr.FromInt(9651)})
	}

	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.Name(),
			Namespace: options.Namespace,
			Labels:    options.K8sConfig.Labels,
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeClusterIP,
			Ports: servicePorts,

			Selector: options.Labels(),
		},
	}
}

func createStatefulSetWithOptions(ctx context.Context, clientset *kubernetes.Clientset, options stateFullSetOptions) error {
	svc := buildService(options)

	serviceClient := clientset.CoreV1().Services(options.Namespace)
	_, foundErr := serviceClient.Get(ctx, options.Name(), metav1.GetOptions{})
	if foundErr == nil {
		_, err := serviceClient.Update(ctx, &svc, metav1.UpdateOptions{
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	} else { //TODO CHECK FOR ACTUAL NOT FOUND ERROR
		_, err := serviceClient.Create(ctx, &svc, metav1.CreateOptions{
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	}

	sts := baseStateFullSet(options)

	stsClient := clientset.AppsV1().StatefulSets(options.Namespace)
	var createdSts *appsv1.StatefulSet
	_, foundErr = stsClient.Get(ctx, options.Name(), metav1.GetOptions{})
	var err error
	if foundErr == nil {
		createdSts, err = stsClient.Update(ctx, &sts, metav1.UpdateOptions{
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	} else { //TODO CHECK FOR ACTUAL NOT FOUND ERROR
		createdSts, err = stsClient.Create(ctx, &sts, metav1.CreateOptions{
			FieldManager: FIELD_MANAGER_STRING,
		})
		if err != nil {
			return err
		}
	}

	if createdSts.Status.AvailableReplicas != options.Replicas {
		watch, err := stsClient.Watch(ctx, metav1.SingleObject(createdSts.ObjectMeta))
		if err != nil {
			return err
		}
		for event := range watch.ResultChan() {
			sts, ok := event.Object.(*appsv1.StatefulSet)
			if !ok {
				fmt.Println("could not parse", event)
				continue
			}

			if sts.Status.AvailableReplicas == options.Replicas && sts.Status.UpdatedReplicas == options.Replicas {
				watch.Stop()
			}

			fmt.Printf("waiting for %s to reach desired state [%d/%d]\n", options.Type, sts.Status.AvailableReplicas, options.Replicas)
		}

	}

	return nil

}

func baseStateFullSet(options stateFullSetOptions) appsv1.StatefulSet {

	labels := options.Labels()

	initContainers := make([]corev1.Container, 0)

	if options.IsValidator {
		initContainers = append(initContainers, validatorInitContainer(options))
	}

	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.Name(),
			Namespace: options.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Replicas:            &options.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: "gcr-image-pull",
						},
					},
					ServiceAccountName: options.PrefixWith("init-container"),
					InitContainers:     initContainers,
					Containers: []corev1.Container{
						buildContainer(options),
					},
					Volumes: defaultVolumes(options.K8sConfig),
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data-vol",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("10Gi"),
							},
						},
					},
				},
			},
		},
	}

}

func buildContainer(options stateFullSetOptions) corev1.Container {

	ports := []corev1.ContainerPort{
		{
			Name:          "rpc",
			ContainerPort: 9650,
		},
	}

	volumeMounts := defaultVolumeMounts()

	if options.IsValidator {
		ports = append(ports, corev1.ContainerPort{Name: "staking", ContainerPort: 9651})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "cert-vol",
			MountPath: "/mnt/cert",
		})
	}

	container := corev1.Container{
		Name:  "camino-node",
		Image: options.Image,
		Resources: corev1.ResourceRequirements{
			Requests: options.Requests,
		},
		Env: []corev1.EnvVar{
			{
				Name: "ROOT_NODE_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: fmt.Sprintf("%s-0", options.K8sPrefix),
						},
						Key: "Node-ID",
					},
				},
			},
			{
				Name:  "NETWORK_NAME",
				Value: options.K8sPrefix,
			},
			{
				Name:  "IS_API_NODE",
				Value: strconv.FormatBool(!(options.IsValidator || options.IsRoot)),
			},
			{
				Name: "POD_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.podIP",
					},
				},
			},
			{
				Name:  "IS_ROOT",
				Value: strconv.FormatBool(options.IsValidator && options.IsRoot),
			},
		},
		Command: []string{
			"bash", "/mnt/scripts/start.sh",
		},
		Ports:        ports,
		VolumeMounts: volumeMounts,
	}

	return container
}

func defaultVolumes(k8sConfig version1.K8sConfig) []corev1.Volume {

	defaultMode := int32(0555)

	return []corev1.Volume{
		{Name: "conf-vol", VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: k8sConfig.K8sPrefix,
				},
				DefaultMode: &defaultMode,
			},
		}},
		{Name: "scripts-vol", VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: fmt.Sprintf("%s-scripts", k8sConfig.K8sPrefix),
				},
				DefaultMode: &defaultMode,
			},
		}},
		{
			Name: "cert-vol", VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}},
	}
}

func defaultVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "conf-vol",
			MountPath: "/mnt/conf",
			ReadOnly:  true,
		},
		{
			Name:      "data-vol",
			MountPath: "/mnt/data",
		},
		{
			Name:      "scripts-vol",
			MountPath: "/mnt/scripts",
		},
	}
}

func validatorInitContainer(options stateFullSetOptions) corev1.Container {

	envs := []corev1.EnvVar{
		{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name:  "SECRET_PREFIX",
			Value: options.K8sPrefix,
		},
	}

	if options.IsRoot {
		envs = append(envs, corev1.EnvVar{
			Name:  "IS_ROOT",
			Value: "true",
		})
	}

	return corev1.Container{
		Name:  "init-certificates",
		Image: "bitnami/kubectl:latest",
		Env:   envs,
		Command: []string{
			"bash", "/mnt/scripts/init.sh",
		},
		VolumeMounts: append(defaultVolumeMounts(), corev1.VolumeMount{
			Name:      "cert-vol",
			MountPath: "/mnt/cert",
			ReadOnly:  false,
		}),
	}
}
