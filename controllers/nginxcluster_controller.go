/*
Copyright 2025.

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

package controllers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nginxv1 "github.com/example/nginx-operator/api/v1"
)

const (
	nginxClusterFinalizer = "nginx.example.com/finalizer"
	configMapNameSuffix   = "-nginx-config"
)

// NginxClusterReconciler reconciles a NginxCluster object
type NginxClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=nginx.example.com,resources=nginxclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nginx.example.com,resources=nginxclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nginx.example.com,resources=nginxclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NginxClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the NginxCluster instance
	nginxCluster := &nginxv1.NginxCluster{}
	err := r.Get(ctx, req.NamespacedName, nginxCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			logger.Info("NginxCluster resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get NginxCluster")
		return ctrl.Result{}, err
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(nginxCluster, nginxClusterFinalizer) {
		controllerutil.AddFinalizer(nginxCluster, nginxClusterFinalizer)
		err = r.Update(ctx, nginxCluster)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check if the NginxCluster instance is marked to be deleted
	isNginxClusterMarkedToBeDeleted := nginxCluster.GetDeletionTimestamp() != nil
	if isNginxClusterMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(nginxCluster, nginxClusterFinalizer) {
			// Run finalization logic
			if err := r.finalizeNginxCluster(ctx, nginxCluster); err != nil {
				return ctrl.Result{}, err
			}

			// Remove finalizer
			controllerutil.RemoveFinalizer(nginxCluster, nginxClusterFinalizer)
			err := r.Update(ctx, nginxCluster)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Calculate config hash
	configHash := calculateConfigHash(nginxCluster.Spec.NginxConf)

	// Check if ConfigMap already exists, if not create a new one
	configMap := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: nginxCluster.Name + configMapNameSuffix, Namespace: nginxCluster.Namespace}, configMap)
	if err != nil && errors.IsNotFound(err) {
		// Define a new ConfigMap
		cm := r.configMapForNginxCluster(nginxCluster, configHash)
		logger.Info("Creating a new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
		err = r.Create(ctx, cm)
		if err != nil {
			logger.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
			return ctrl.Result{}, err
		}
	} else if err != nil {
		logger.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	} else {
		// ConfigMap exists, check if config has changed
		currentConfigHash := configMap.Annotations["config-hash"]
		if currentConfigHash != configHash {
			logger.Info("Configuration changed, updating ConfigMap and triggering restart")
			configMap.Data["nginx.conf"] = nginxCluster.Spec.NginxConf
			configMap.Annotations["config-hash"] = configHash
			err = r.Update(ctx, configMap)
			if err != nil {
				logger.Error(err, "Failed to update ConfigMap")
				return ctrl.Result{}, err
			}
		}
	}

	// Check if the Deployment already exists, if not create a new one
	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: nginxCluster.Name, Namespace: nginxCluster.Namespace}, deployment)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForNginxCluster(nginxCluster, configHash)
		logger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			logger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Ensure the deployment replicas is the same as the spec
	replicas := nginxCluster.Spec.Replicas
	if *deployment.Spec.Replicas != replicas {
		deployment.Spec.Replicas = &replicas
		err = r.Update(ctx, deployment)
		if err != nil {
			logger.Error(err, "Failed to update Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// Check if config has changed and trigger rolling update
	currentPodConfigHash := deployment.Spec.Template.Annotations["config-hash"]
	if currentPodConfigHash != configHash {
		logger.Info("Configuration changed, triggering rolling update of pods")
		deployment.Spec.Template.Annotations["config-hash"] = configHash
		// Update restart timestamp to force pod recreation
		deployment.Spec.Template.Annotations["restartedAt"] = time.Now().Format(time.RFC3339)
		err = r.Update(ctx, deployment)
		if err != nil {
			logger.Error(err, "Failed to update Deployment for config change")
			return ctrl.Result{}, err
		}
	}

	// Check if the Service already exists, if not create a new one
	service := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: nginxCluster.Name, Namespace: nginxCluster.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		// Define a new service
		srv := r.serviceForNginxCluster(nginxCluster)
		logger.Info("Creating a new Service", "Service.Namespace", srv.Namespace, "Service.Name", srv.Name)
		err = r.Create(ctx, srv)
		if err != nil {
			logger.Error(err, "Failed to create new Service", "Service.Namespace", srv.Namespace, "Service.Name", srv.Name)
			return ctrl.Result{}, err
		}
	} else if err != nil {
		logger.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// Update the NginxCluster status
	nginxCluster.Status.Replicas = deployment.Status.Replicas
	nginxCluster.Status.ReadyReplicas = deployment.Status.ReadyReplicas
	nginxCluster.Status.ConfigHash = configHash
	now := metav1.Now()
	nginxCluster.Status.LastUpdateTime = &now

	err = r.Status().Update(ctx, nginxCluster)
	if err != nil {
		logger.Error(err, "Failed to update NginxCluster status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// configMapForNginxCluster returns a ConfigMap object
func (r *NginxClusterReconciler) configMapForNginxCluster(m *nginxv1.NginxCluster, configHash string) *corev1.ConfigMap {
	nginxConf := m.Spec.NginxConf
	if nginxConf == "" {
		nginxConf = getDefaultNginxConf()
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name + configMapNameSuffix,
			Namespace: m.Namespace,
			Annotations: map[string]string{
				"config-hash": configHash,
			},
		},
		Data: map[string]string{
			"nginx.conf": nginxConf,
		},
	}
	// Set NginxCluster instance as the owner and controller
	ctrl.SetControllerReference(m, cm, r.Scheme)
	return cm
}

// deploymentForNginxCluster returns a Deployment object
func (r *NginxClusterReconciler) deploymentForNginxCluster(m *nginxv1.NginxCluster, configHash string) *appsv1.Deployment {
	replicas := m.Spec.Replicas
	image := m.Spec.Image
	if image == "" {
		image = "nginx:latest"
	}

	labels := map[string]string{
		"app":     "nginx",
		"cluster": m.Name,
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"config-hash": configHash,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: image,
						Name:  "nginx",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 80,
							Name:          "http",
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "nginx-config",
							MountPath: "/etc/nginx/nginx.conf",
							SubPath:   "nginx.conf",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "nginx-config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: m.Name + configMapNameSuffix,
								},
							},
						},
					}},
				},
			},
		},
	}
	// Set NginxCluster instance as the owner and controller
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// serviceForNginxCluster returns a Service object
func (r *NginxClusterReconciler) serviceForNginxCluster(m *nginxv1.NginxCluster) *corev1.Service {
	labels := map[string]string{
		"app":     "nginx",
		"cluster": m.Name,
	}

	srv := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Port:     80,
				Name:     "http",
				Protocol: corev1.ProtocolTCP,
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	// Set NginxCluster instance as the owner and controller
	ctrl.SetControllerReference(m, srv, r.Scheme)
	return srv
}

func (r *NginxClusterReconciler) finalizeNginxCluster(ctx context.Context, m *nginxv1.NginxCluster) error {
	logger := log.FromContext(ctx)
	logger.Info("Successfully finalized nginxCluster")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NginxClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nginxv1.NginxCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// calculateConfigHash calculates a hash of the nginx configuration
func calculateConfigHash(config string) string {
	hash := sha256.Sum256([]byte(config))
	return fmt.Sprintf("%x", hash)[:16]
}

// getDefaultNginxConf returns default nginx configuration
func getDefaultNginxConf() string {
	return `
events {
    worker_connections 1024;
}

http {
    include       /etc/nginx/mime.types;
    default_type  application/octet-stream;

    sendfile        on;
    keepalive_timeout  65;

    server {
        listen       80;
        server_name  localhost;

        location / {
            root   /usr/share/nginx/html;
            index  index.html index.htm;
        }

        error_page   500 502 503 504  /50x.html;
        location = /50x.html {
            root   /usr/share/nginx/html;
        }
    }
}
`
}

