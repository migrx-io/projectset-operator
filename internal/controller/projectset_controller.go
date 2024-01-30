/*
Copyright 2024 Anatolii Makarov.

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
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/migrx-io/projectset-operator/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logger "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	projectv1alpha1 "github.com/migrx-io/projectset-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Finilizer name
const projectSetFinalizer = "projectsets.project.migrx.io/finalizer"

// Definitions to manage status conditions
const (
	// Resource is Available
	typeAvailableStatus = "Available"
	typeDegradedStatus  = "Degraded"
)

// ProjectSetReconciler reconciles a ProjectSet object
type ProjectSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsets/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=resourcequotas,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=core,resources=limitranges,verbs=get;list;watch;create;update;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ProjectSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile

var log logr.Logger

func (r *ProjectSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log = logger.FromContext(ctx).WithValues("projectset", req.NamespacedName)

	//
	// Fetch the instance
	//
	instance := &projectv1alpha1.ProjectSet{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("ProjectSet resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get ProjectSet")
		return ctrl.Result{}, err
	}

	//
	// Set the status as Unknown when no status are available
	//
	if instance.Status.Conditions == nil || len(instance.Status.Conditions) == 0 {

		if err := r.setStatus(ctx, req, instance,
			typeAvailableStatus,
			metav1.ConditionUnknown,
			"Reconciling",
			"Starting reconciliation"); err != nil {

			return ctrl.Result{}, err

		}

	}

	//
	// Add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	//
	if !controllerutil.ContainsFinalizer(instance, projectSetFinalizer) {

		log.Info("Adding Finalizer")
		if ok := controllerutil.AddFinalizer(instance, projectSetFinalizer); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

	}

	//
	// Check if the instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	//
	isMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil

	if isMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(instance, projectSetFinalizer) {

			log.Info("Performing Finalizer Operations before delete CR")

			// Update status
			if err := r.setStatus(ctx, req, instance,
				typeDegradedStatus,
				metav1.ConditionUnknown,
				"Finalizing",
				"Performing finilizer operations"); err != nil {

				return ctrl.Result{}, err

			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			r.doFinalizerOperations(instance)

			// Update status
			if err := r.setStatus(ctx, req, instance,
				typeDegradedStatus,
				metav1.ConditionTrue,
				"Finalizing",
				"Finilizer operations are done"); err != nil {

				return ctrl.Result{}, err

			}

			// Remove finilizers
			log.Info("Removing Finalizer after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(instance, projectSetFinalizer); !ok {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, instance); err != nil {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}
	}

	//
	// Namespace Logic
	//

	namespaceFound := &corev1.Namespace{}

	err = r.Get(ctx, types.NamespacedName{Name: instance.Spec.Namespace}, namespaceFound)

	if err != nil && apierrors.IsNotFound(err) {

		namespace, err := r.defineNamespace(ctx, req, instance)

		if err != nil {
			log.Error(err, "Failed to define namespace")
			return ctrl.Result{}, err
		}

		// Create namespace
		if err := r.createNamespace(ctx, req, instance, namespace); err != nil {
			return ctrl.Result{}, err
		}

		// Save event
		r.Recorder.Event(instance,
			"Normal",
			"Create namespace",
			fmt.Sprintf("Namespace %s created", instance.Spec.Namespace))

		return ctrl.Result{Requeue: true}, nil

	} else if err != nil {

		log.Error(err, "Failed to get Namespace")

		// Reconcile failed due to error - requeue
		return ctrl.Result{}, err

	}

	//
	// Object exists - compare states
	//

	// Check namaspace is chnaged
	if err := r.checkAndUpdateNamespace(ctx, req, instance, namespaceFound); err != nil {
		return ctrl.Result{}, err
	}

	// Check reqource quota
	rq, err := r.createAndUpdateResourceQuota(ctx, req, instance, namespaceFound)

	if err != nil {
		return ctrl.Result{}, err

	} else if rq != nil && err == nil {
		// if rq was created or changed
		return ctrl.Result{Requeue: true}, nil
	}

	// Update status if all complete
	if err := r.setStatus(ctx, req, instance,
		typeAvailableStatus,
		metav1.ConditionTrue,
		"Reconciling",
		"Reconciling is done"); err != nil {

		return ctrl.Result{}, err

	}

	return ctrl.Result{}, nil
}

//
//
// Reconcile helper functions
//
//

// Namespace build logic
func (r *ProjectSetReconciler) namespaceForProjectSet(projSet *projectv1alpha1.ProjectSet) (*corev1.Namespace, error) {

	labels := projSet.Spec.Labels
	annotations := projSet.Spec.Annotations

	// add projectset-name as annotation for all child resources for correct watching
	annotations["projectset-name"] = projSet.GetName()

	log.Info("namespaceForProject", "labels", labels, "annotations", annotations)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   projSet.Spec.Namespace,
			Name:        projSet.Spec.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
	}

	// Set the ownerRef for the Namespace
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(projSet, namespace, r.Scheme); err != nil {
		return nil, err
	}

	return namespace, nil
}

// Resource Quota build logic
func (r *ProjectSetReconciler) resourceQuotaForNamespace(namespace *corev1.Namespace, projSet *projectv1alpha1.ProjectSet) (*corev1.ResourceQuota, error) {

	labels := namespace.GetLabels()
	annotations := namespace.GetAnnotations()

	resourceQuota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace.Name,
			Name:        namespace.Name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: projSet.Spec.ResourceQuota.Hard,
		},
	}

	// Set the ownerRef for the Namespace
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(namespace, resourceQuota, r.Scheme); err != nil {
		return nil, err
	}

	return resourceQuota, nil
}

// Resource Quota logic create/update
// Return only requeue and errors
func (r *ProjectSetReconciler) createAndUpdateResourceQuota(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace) (*corev1.ResourceQuota, error) {

	//check if defined in instance
	if instance.Spec.ResourceQuota.Hard == nil {
		log.Info("Resource quota is not defined")
		return nil, nil
	}

	// Find if resourcequota exists
	resourceQuotaFound := &corev1.ResourceQuota{}
	err := r.Get(ctx, types.NamespacedName{Name: namespace.Name, Namespace: namespace.Name}, resourceQuotaFound)

	if err != nil && apierrors.IsNotFound(err) {

		// define a new resourcequota
		rq, err := r.resourceQuotaForNamespace(namespace, instance)

		if err != nil {
			log.Error(err, "Failed to define new ResourceQuota")
			return nil, err
		}

		log.Info("Creating a new ResourceQuota", "ResourceQuota.Namespace", rq.Namespace, "ResourceQuota.Name", rq.Name)

		err = r.Create(ctx, rq)

		if err != nil {
			log.Error(err, "Failed to create new ResourceQuota", "ResourceQuota.Namespace", rq.Namespace, "ResourceQuota.Name", rq.Name)
			return nil, err
		}

		// Save event
		r.Recorder.Event(instance,
			"Normal",
			"Create ResourceQuota",
			fmt.Sprintf("ResourceQuota %s created", rq.Name))

		// resourcequota created, return and requeue
		return rq, nil

	} else if err != nil {

		log.Error(err, "Failed to get ResourceQuota")
		// Reconcile failed due to error - requeue
		return nil, err
	}

	log.Info("ResourceQuota exists")

	return nil, nil

}

// Get default limits limitRange
// hack to avoid validation issue
func (r *ProjectSetReconciler) getOrDefaultLimitRange(instance *projectv1alpha1.ProjectSet) []corev1.LimitRangeItem {

	log.Info("Ckeck limits in LimitRange and set")

	if instance.Spec.LimitRange.Limits == nil {

		limits := []corev1.LimitRangeItem{}

		log.Info("LimitRange is not defined. Create stub")

		return limits

	}

	return instance.Spec.LimitRange.Limits

}

// Define new LimitRange
func (r *ProjectSetReconciler) limitRangeForNamespace(instance *projectv1alpha1.ProjectSet, namespace *corev1.Namespace) *corev1.LimitRange {

	labels := namespace.GetLabels()
	annotations := namespace.GetAnnotations()
	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace.Name,
			Name:        namespace.Name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.LimitRangeSpec{
			Limits: r.getOrDefaultLimitRange(instance),
		},
	}

	return limitRange
}

// Check namespace changes with ProjectSet
func (r *ProjectSetReconciler) checkAndUpdateNamespace(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace) error {

	log.Info("labels", "namespace", namespace.ObjectMeta.Labels, "instance", instance.Spec.Labels)
	log.Info("annotations", "namespace", namespace.ObjectMeta.Annotations, "instance", instance.Spec.Annotations)

	// if label or annotations changed - update namespace
	// apiequality.Semantic.DeepDerivative(desiredObj.Spec, runtimeObj.Spec)
	if !utils.IsMapSubset(namespace.ObjectMeta.Labels, instance.Spec.Labels) ||
		!utils.IsMapSubset(namespace.ObjectMeta.Annotations, instance.Spec.Annotations) {

		log.Info("Namespace labels or annotations are dirreferent - update from instance")

		namespace.ObjectMeta.Labels = instance.Spec.Labels
		namespace.ObjectMeta.Annotations = instance.Spec.Annotations

		if err := r.Update(ctx, namespace); err != nil {

			// Update status
			if err := r.setStatus(ctx, req, instance,
				typeAvailableStatus,
				metav1.ConditionFalse,
				"Reconciling",
				fmt.Sprintf("Failed to create namespace %s", namespace.Name)); err != nil {

				return err

			}

			return err
		}

		// Save event
		r.Recorder.Event(instance,
			"Normal",
			"Update namespace",
			fmt.Sprintf("Namespace %s updated", instance.Spec.Namespace))

	}

	return nil

}

// Define new namespace based on ProjectSet
func (r *ProjectSetReconciler) createNamespace(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace) error {

	log.Info("Creating a new Namespace", "Namespace.Name", namespace.Name)

	if err := r.Create(ctx, namespace); err != nil {
		log.Error(err, "Failed to create new Namespace", "Namespace.Name", namespace.Name)

		// Update status
		if err := r.setStatus(ctx, req, instance,
			typeAvailableStatus,
			metav1.ConditionFalse,
			"Reconciling",
			fmt.Sprintf("Failed to create namespace %s", namespace.Name)); err != nil {

			return err

		}

		return err
	}

	// namespace created, return and requeue
	log.Info("Namespace created", "Namespace.Name", namespace.Name)

	return nil

}

// Define new namespace based on ProjectSet
func (r *ProjectSetReconciler) defineNamespace(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet) (*corev1.Namespace, error) {

	// Define a new namespace
	namespace, err := r.namespaceForProjectSet(instance)

	log.Info(fmt.Sprintf("Define new namespace: %s", namespace))

	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to create namespace %s", instance.Spec.Namespace))

		// Update status
		if err := r.setStatus(ctx, req, instance,
			typeAvailableStatus,
			metav1.ConditionFalse,
			"Reconciling",
			fmt.Sprintf("Failed to define namespace %s", instance.Spec.Namespace)); err != nil {

			return nil, err

		}

		return nil, err

	}

	return namespace, nil

}

func (r *ProjectSetReconciler) setStatus(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	statusType string,
	status metav1.ConditionStatus,
	reason, message string) error {

	// Refetch last state
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "Failed to re-fetch instance")
		return err
	}

	// Set condition
	meta.SetStatusCondition(&instance.Status.Conditions,
		metav1.Condition{Type: statusType,
			Status:  status,
			Reason:  reason,
			Message: message})

	// Update state
	if err := r.Status().Update(ctx, instance); err != nil {
		log.Error(err, "Failed to update instance status")
		return err
	}

	// FIXME, hack to resolve validation issue, implement default vaules in webhook
	// Patch default LimitRange to fix issue with required fields
	instance.Spec.LimitRange.Limits = r.getOrDefaultLimitRange(instance)

	if err := r.Update(ctx, instance); err != nil {
		log.Error(err, "Failed to update instance")
		return err
	}

	// Refetch last state
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "Failed to re-fetch instance")
		return err
	}

	return nil
}

// Finalizers will perform the required operations before delete the CR.
func (r *ProjectSetReconciler) doFinalizerOperations(cr *projectv1alpha1.ProjectSet) {
	// TODO(user): Add the cleanup steps that the operator
	// needs to do before the CR can be deleted. Examples
	// of finalizers include performing backups and deleting
	// resources that are not owned by this CR, like a PVC.

	// Note: It is not recommended to use finalizers with the purpose of delete resources which are
	// created and managed in the reconciliation. These ones, such as the Deployment created on this reconcile,
	// are defined as depended of the custom resource. See that we use the method ctrl.SetControllerReference.
	// to set the ownerRef which means that the Deployment will be deleted by the Kubernetes API.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/

	//eventtype is the type of this event, and is either Normal or Warning.
	// The following implementation will raise an event

	r.Recorder.Event(cr,
		"Warning",
		"Deleting",
		fmt.Sprintf("Custom Resource %s is being deleted", cr.Name))

}

// Finalizers will perform the required operations before delete the CR.
func (r *ProjectSetReconciler) findProjectSetByResourceQuotaName(ctx context.Context, rq *corev1.ResourceQuota) ([]projectv1alpha1.ProjectSet, error) {

	psFound := &projectv1alpha1.ProjectSet{}

	err := r.Get(ctx, types.NamespacedName{Name: rq.GetAnnotations()["projectset-name"]}, psFound)

	// if returned empty struct
	if name := psFound.GetName(); name == "" {
		return nil, fmt.Errorf("ProjectSet already deleted")
	}

	log.Info("findProjectSetByResourceQuotaName", "err", err, "rq", rq, "psFound", psFound)

	if err != nil {
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get ProjectSet")
		return nil, err
	}

	return []projectv1alpha1.ProjectSet{*psFound}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projectv1alpha1.ProjectSet{}, builder.WithPredicates(utils.ResourceGenerationOrFinalizerChangedPredicate{})).
		Owns(&corev1.Namespace{}).
		Watches(&corev1.ResourceQuota{
			TypeMeta: metav1.TypeMeta{
				Kind: "ResourceQuota",
			},
		}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}

			rq := a.(*corev1.ResourceQuota)

			projSet, err := r.findProjectSetByResourceQuotaName(ctx, rq)

			if err != nil {
				return []reconcile.Request{}
			}

			for _, config := range projSet {
				reconcileRequests = append(reconcileRequests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      config.GetName(),
						Namespace: config.GetNamespace(),
					},
				})
			}

			log.Info("reconcileRequests", "request", reconcileRequests)

			return reconcileRequests
		})).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
