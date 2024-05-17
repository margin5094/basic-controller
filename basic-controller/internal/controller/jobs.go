package controller

import (
	v1 "github.com/margin5094/basic-controller/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func constructJobForTool(clusterScan v1.ClusterScan) *batchv1.Job {
	var containerImage string
	var containerCommand []string

	switch clusterScan.Spec.Tool {
	case "kube-bench":
		containerImage = "aquasec/kube-bench:latest"
		containerCommand = []string{"kube-bench"}
	case "kube-hunter":
		containerImage = "aquasec/kube-hunter:latest"
		containerCommand = []string{"kube-hunter", "--pod"}
	default:

	}

	labels := map[string]string{
		"clusterScanName": clusterScan.Name,
		"toolName":        clusterScan.Spec.Tool,
		"type":            "cluster-scan",
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: clusterScan.Name + "-",
			Namespace:    clusterScan.Namespace,
			Labels:       labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    clusterScan.Spec.Tool,
							Image:   containerImage,
							Command: containerCommand,
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	return job
}

func constructCronJobForTool(clusterScan v1.ClusterScan) *batchv1.CronJob {
	var containerImage string
	var containerCommand []string

	switch clusterScan.Spec.Tool {
	case "kube-bench":
		containerImage = "aquasec/kube-bench:latest"
		containerCommand = []string{"kube-bench"}
	case "kube-hunter":
		containerImage = "aquasec/kube-hunter:latest"
		containerCommand = []string{"kube-hunter", "--pod"}
	default:

	}

	labels := map[string]string{
		"clusterScanName": clusterScan.Name,
		"toolName":        clusterScan.Spec.Tool,
		"type":            "cluster-scan",
	}

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: clusterScan.Name + "-",
			Namespace:    clusterScan.Namespace,
			Labels:       labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: clusterScan.Spec.Schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:    clusterScan.Spec.Tool,
									Image:   containerImage,
									Command: containerCommand,
								},
							},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}

	return cronJob
}
