package ironicapi

import (
	"context"
    "reflect"

	ironicv1alpha1 "github.com/redhat-nfvpe/ironic-operator/pkg/apis/ironic/v1alpha1"
    helpers "github.com/redhat-nfvpe/ironic-operator/pkg/helpers"

  	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
    "k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
    "sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_ironicapi")

// Add creates a new IronicApi Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileIronicApi{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("ironicapi-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource IronicApi
	err = c.Watch(&source.Kind{Type: &ironicv1alpha1.IronicApi{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &ironicv1alpha1.IronicApi{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileIronicApi{}

// ReconcileIronicApi reconciles a IronicApi object
type ReconcileIronicApi struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a IronicApi object and makes changes based on the state read
// and what is in the IronicApi.Spec
func (r *ReconcileIronicApi) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling IronicApi")

	// Fetch the IronicApi instance
	instance := &ironicv1alpha1.IronicApi{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

    // Check if the configmap already exists, if not create a new one
    cm_found := &corev1.ConfigMap{}
    err = r.client.Get(context.TODO(), types.NamespacedName{Name: "ironic-bin", Namespace: instance.Namespace}, cm_found)
    if err != nil && errors.IsNotFound(err) {
        // define a new configmap
        cm, _ := helpers.GetIronicBinConfigMap(instance.Namespace)
        reqLogger.Info("Creating a new ironic-bin configmap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
        err = r.client.Create(context.TODO(), cm)
        if err != nil {
            reqLogger.Error(err, "failed to create a new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
            return reconcile.Result{}, err
        }
    } else if err != nil {
        reqLogger.Error(err, "failed to get ironic-bin ConfigMap")
        return reconcile.Result{}, err
    }

    cm_etc_found := &corev1.ConfigMap{}
    err = r.client.Get(context.TODO(), types.NamespacedName{Name: "ironic-etc", Namespace: instance.Namespace}, cm_etc_found)
    if err != nil && errors.IsNotFound(err) {
        // define a new configmap
        cm_etc, _ := helpers.GetIronicEtcConfigMap(instance.Namespace, r.client)
        reqLogger.Info("Creating a new ironic-etc configmap", "ConfigMap.Namespace", cm_etc.Namespace, "ConfigMap.Name", cm_etc.Name)
        err = r.client.Create(context.TODO(), cm_etc)
        if err != nil {
            reqLogger.Error(err, "failed to create a new ConfigMap", "ConfigMap.Namespace", cm_etc.Namespace, "ConfigMap.Name", cm_etc.Name)
            return reconcile.Result{}, err
        }
    } else if err != nil {
        reqLogger.Error(err, "failed to get ironic-etc ConfigMap")
        return reconcile.Result{}, err
    }

    // retrieve entries in configmap for images
    cm_images := &corev1.ConfigMap{}
    err = r.client.Get(context.TODO(), types.NamespacedName{Name: "images", Namespace: instance.Namespace}, cm_images)

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForIronicApi(instance, cm_images.Data)
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "failed to get Deployment")
		return reconcile.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	size := instance.Spec.Size
	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		err = r.client.Update(context.TODO(), found)
		if err != nil {
			reqLogger.Error(err, "failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return reconcile.Result{}, err
		}
		// Spec updated - return and requeue
		return reconcile.Result{Requeue: true}, nil
	}

    // Check if the service already exists, if not create a new one
    found_srv := &corev1.Service{}
    err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found_srv)
    if err != nil && errors.IsNotFound(err) {
        // Define a new service
        srv := r.serviceForIronicApi(instance)

		reqLogger.Info("Creating a new Service", "Service.Namespace", srv.Namespace, "Service.Name", srv.Name)
		err = r.client.Create(context.TODO(), srv)
		if err != nil {
			reqLogger.Error(err, "failed to create new Service", "Service.Namespace", srv.Namespace, "Service.Name", srv.Name)
			return reconcile.Result{}, err
		}
		// Service created successfully - return and requeue
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "failed to get Service")
		return reconcile.Result{}, err
    }


	// Update the Ironic Api status with the pod names
	// List the pods for this ironic api's deployment
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForIronicApi(instance.Name))
	listOps := &client.ListOptions{Namespace: instance.Namespace, LabelSelector: labelSelector}
	err = r.client.List(context.TODO(), listOps, podList)
	if err != nil {
		reqLogger.Error(err, "failed to list pods", "IronicApi.Namespace", instance.Namespace, "IronicApi.Name", instance.Name)
		return reconcile.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, instance.Status.Nodes) {
		instance.Status.Nodes = podNames
		err := r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "failed to update IronicApi status")
			return reconcile.Result{}, err
		}
	}

    return reconcile.Result{}, nil
}

// deploymentForIronicApi returns a ironic-api Deployment object
func (r *ReconcileIronicApi) deploymentForIronicApi(m *ironicv1alpha1.IronicApi, images map[string]string) *appsv1.Deployment {
	ls := labelsForIronicApi(m.Name)
	replicas := m.Spec.Size

    var readMode int32 = 0444
    var execMode int32 = 0555

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
                    InitContainers: []corev1.Container{{
                        Image: images["KUBERNETES_ENTRYPOINT"],
                        Name: "init",
                        ImagePullPolicy: "IfNotPresent",
                        Env: []corev1.EnvVar{
                            {
                                Name: "POD_NAME",
                                ValueFrom: &corev1.EnvVarSource{
                                    FieldRef: &corev1.ObjectFieldSelector {
                                        APIVersion: "v1",
                                        FieldPath: "metadata.name",
                                    },
                                },
                            }, {
                                Name: "NAMESPACE",
                                ValueFrom: &corev1.EnvVarSource{
                                    FieldRef: &corev1.ObjectFieldSelector{
                                        APIVersion: "v1",
                                        FieldPath: "metadata.namespace",
                                    },
                                },
                            }, {
                                Name: "PATH",
                                Value: "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/",
                            }, {
                                Name: "COMMAND",
                                Value: "echo done",
                            },
                        },
                        Command: []string{"kubernetes-entrypoint"},
                    }},
					Containers: []corev1.Container{{
						Image:   images["IRONIC_API"],
						Name:    "ironic-api",
                        ImagePullPolicy: "IfNotPresent",
						Command: []string{"/tmp/ironic-api.sh", "start"},
						Lifecycle: &corev1.Lifecycle{
                            PreStop: &corev1.Handler{
                                Exec: &corev1.ExecAction{
                                    Command: []string{"/tmp/ironic-api.sh", "stop"},
                                },
                            },
						},
                        Ports: []corev1.ContainerPort{
                            {
                                ContainerPort: 6385,
                            },
                        },
                        ReadinessProbe: &corev1.Probe{
                            Handler: corev1.Handler{
                                TCPSocket: &corev1.TCPSocketAction{
                                    Port: intstr.FromInt(6385),
                                },
                            },
                        },
                        VolumeMounts: []corev1.VolumeMount{
                            {
                                Name: "ironic-bin",
                                MountPath: "/tmp/ironic-api.sh",
                                SubPath: "ironic-api.sh",
                                ReadOnly: true,
                            },
                            {
                                Name: "ironic-etc",
                                MountPath: "/etc/ironic/ironic.conf",
                                SubPath: "ironic.conf",
                                ReadOnly: true,
                            },
                            {
                                Name: "ironic-etc",
                                MountPath: "/etc/ironic/logging.conf",
                                SubPath: "logging.conf",
                                ReadOnly: true,
                            },
                            {
                                Name: "ironic-etc",
                                MountPath: "/etc/ironic/policy.json",
                                SubPath: "policy.json",
                                ReadOnly: true,
                            },
                            {
                                Name: "pod-shared",
                                MountPath: "/tmp/pod-shared",
                            },
                        },
					}},
                    Volumes: []corev1.Volume{
                        {
                            Name: "ironic-bin",
                            VolumeSource: corev1.VolumeSource {
                                ConfigMap: &corev1.ConfigMapVolumeSource {
                                    DefaultMode: &execMode,
				                    LocalObjectReference: corev1.LocalObjectReference{
					                    Name: "ironic-bin",
				                    },
                                },
                            },
                        },
                        {
                            Name: "ironic-etc",
                            VolumeSource: corev1.VolumeSource {
                                ConfigMap: &corev1.ConfigMapVolumeSource {
                                    DefaultMode: &readMode,
				                    LocalObjectReference: corev1.LocalObjectReference{
					                    Name: "ironic-etc",
				                    },
                                },

                            },
                        },
                        {
                            Name: "pod-shared",
                            VolumeSource: corev1.VolumeSource {
                                EmptyDir: &corev1.EmptyDirVolumeSource {},
                            },
                        },
                    },
				},
			},
		},
	}
	// Set Memcached instance as the owner and controller
	controllerutil.SetControllerReference(m, dep, r.scheme)
	return dep
}

// labelsForIronicApi returns the labels for selecting the resources
// belonging to the given ironic api CR name.
func labelsForIronicApi(name string) map[string]string {
	return map[string]string{"app": "ironic", "ironicapi_cr": name}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// serviceForIronicApi returns a ironic-api Service object
func (r *ReconcileIronicApi) serviceForIronicApi(m *ironicv1alpha1.IronicApi) *corev1.Service {

    srv_selector := map[string]string{"app": "ironic", "ironicapi_cr": "openstack-ironicapi"}
	srv := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "core/v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
            Type: "NodePort",
			Selector: srv_selector,
            Ports: []corev1.ServicePort{
                {
                    Name: "ironic-api",
                    Protocol: "TCP",
                    Port: 6385,
                    NodePort: 32732,
                },
            },
		},
	}
	return srv
}


