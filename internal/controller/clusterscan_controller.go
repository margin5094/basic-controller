/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-logr/logr"

	v1 "github.com/margin5094/basic-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ClusterScanReconciler reconciles a ClusterScan object
type ClusterScanReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Clientset *kubernetes.Clientset
}

//+kubebuilder:rbac:groups=security.basiccontroller.com,resources=clusterscans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=security.basiccontroller.com,resources=clusterscans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=security.basiccontroller.com,resources=clusterscans/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterScan object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var clusterScan v1.ClusterScan

	if err := r.Get(ctx, req.NamespacedName, &clusterScan); err != nil {
		log.Error(err, "unable to fetch ClusterScan")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if clusterScan.Status.Results == nil {
		clusterScan.Status.Results = []string{}
	}

	if clusterScan.Status.JobCreated {
		return r.handleCreatedJob(ctx, clusterScan, req, log)
	}

	if clusterScan.Spec.JobType == "CronJob" {
		if clusterScan.Spec.Schedule == "" {
			log.Error(nil, "schedule is required for CronJob")
			return ctrl.Result{}, fmt.Errorf("schedule is required for CronJob")
		}
		cronJob := constructCronJobForTool(clusterScan)
		if err := r.Create(ctx, cronJob); err != nil {
			log.Error(err, "unable to create CronJob for ClusterScan", "cronJob", cronJob)
			updateConditions(&clusterScan.Status.Conditions, "CronJobCreationError", metav1.ConditionFalse, "CreationFailed", "Failed to create CronJob for ClusterScan")
			r.Status().Update(ctx, &clusterScan)
			return ctrl.Result{}, err
		}
	} else {
		job := constructJobForTool(clusterScan)
		if err := r.Create(ctx, job); err != nil {
			log.Error(err, "unable to create Job for ClusterScan", "job", job)
			updateConditions(&clusterScan.Status.Conditions, "JobCreationError", metav1.ConditionFalse, "CreationFailed", "Failed to create Job for ClusterScan")
			r.Status().Update(ctx, &clusterScan)
			return ctrl.Result{}, err
		}
	}

	clusterScan.Status.JobCreated = true
	updateConditions(&clusterScan.Status.Conditions, "JobCreated", metav1.ConditionTrue, "CreationSuccessful", "Job created successfully")
	if err := r.Status().Update(ctx, &clusterScan); err != nil {
		log.Error(err, "failed to update ClusterScan status")
		return ctrl.Result{}, err
	}

	log.Info("Job/CronJob created and ClusterScan status updated", "ClusterScan", clusterScan.Name)
	return ctrl.Result{}, nil
}

func (r *ClusterScanReconciler) handleCreatedJob(ctx context.Context, clusterScan v1.ClusterScan, req ctrl.Request, log logr.Logger) (ctrl.Result, error) {
	var pods corev1.PodList
	labelSelector := client.MatchingLabels{
		"clusterScanName": clusterScan.Name,
		"toolName":        clusterScan.Spec.Tool,
	}
	if err := r.List(ctx, &pods, client.InNamespace(req.Namespace), labelSelector); err != nil {
		log.Error(err, "failed to list pods for the job")
		return ctrl.Result{}, err
	}

	for _, pod := range pods.Items {
		if pod.DeletionTimestamp != nil {
			log.Info("Pod is in the process of being deleted, skipping", "PodName", pod.Name)
			continue
		}

		if pod.Status.Phase == corev1.PodSucceeded {
			logs, err := r.getPodLogs(pod)
			// log.Info("Logs fetched from pod", "PodName", pod.Name, "Logs", logs)
			if err != nil {
				log.Error(err, "failed to get logs from pod", "PodName", pod.Name)
				return ctrl.Result{}, err
			}

			clusterScan.Status.Results = append(clusterScan.Status.Results, logs)
			clusterScan.Status.Completed = true
			clusterScan.Status.Success = true
			if err := r.Status().Update(ctx, &clusterScan); err != nil {
				log.Error(err, "failed to update ClusterScan status with logs")
				return ctrl.Result{}, err
			}

			if err := r.Delete(ctx, &pod); err != nil {
				log.Error(err, "failed to delete pod", "PodName", pod.Name)
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *ClusterScanReconciler) getPodLogs(pod corev1.Pod) (string, error) {
	podLogOpts := corev1.PodLogOptions{}
	req := r.Clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.Background())
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ClusterScan{}).
		Complete(r)
}
