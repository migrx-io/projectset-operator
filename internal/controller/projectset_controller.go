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
	"k8s.io/apimachinery/pkg/api/equality"
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
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsettemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsets/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=resourcequotas,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=core,resources=limitranges,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=roles.rbac.authorization.k8s.io,resources=*,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=*,resources=*,verbs=*

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
	// Check template if exists
	//

	if err := r.checkAndUpdateTemplate(ctx, req, instance); err != nil {
		return ctrl.Result{}, err
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

	//
	// Check namaspace is chnaged
	//
	if err := r.checkAndUpdateNamespace(ctx, req, instance, namespaceFound); err != nil {
		return ctrl.Result{}, err
	}

	//
	// Check reqource quota
	//
	rq, err := r.createAndUpdateResourceQuota(ctx, req, instance, namespaceFound)

	if err != nil {
		return ctrl.Result{}, err

	} else if rq != nil && err == nil {
		// if rq was created or changed
		return ctrl.Result{Requeue: true}, nil
	}

	//
	// Check limit range
	//
	lr, err := r.createAndUpdateLimitRange(ctx, req, instance, namespaceFound)

	if err != nil {
		return ctrl.Result{}, err

	} else if lr != nil && err == nil {
		// if rq was created or changed
		return ctrl.Result{Requeue: true}, nil
	}

	//
	// Check network policies
	//
	np, err := r.createAndUpdateNetworkPolicies(ctx, req, instance, namespaceFound)

	if err != nil {
		return ctrl.Result{}, err

	} else if np != nil && err == nil {
		// if rq was created or changed
		return ctrl.Result{Requeue: true}, nil
	}

	//
	// Check role rules
	//
	rl, err := r.createAndUpdateRoles(ctx, req, instance, namespaceFound)

	if err != nil {
		return ctrl.Result{}, err

	} else if rl != nil && err == nil {
		// if rq was created or changed
		return ctrl.Result{Requeue: true}, nil
	}

	//
	// Check group permissions
	//
	rb, err := r.createAndUpdateRoleBindings(ctx, req, instance, namespaceFound)

	if err != nil {
		return ctrl.Result{}, err

	} else if rb != nil && err == nil {
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

func (r *ProjectSetReconciler) checkAndDeleteRoleBind(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace,
	name string) error {

	// Find if limitrange exists
	roleRuleFound := &rbacv1.RoleBinding{}

	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace.Name}, roleRuleFound)

	if err != nil && apierrors.IsNotFound(err) {

		return nil

	} else if err != nil {

		log.Error(err, "Failed to get RoleBinding")
		// Reconcile failed due to error - requeue
		return err
	}

	// delete rule because not found in CR
	err = r.Delete(ctx, roleRuleFound)

	if err != nil {
		log.Error(err, "Failed to delete RoleBinding", "Namespace", roleRuleFound.Namespace, "Name", roleRuleFound.Name)
		return err
	}

	// Save event
	r.Recorder.Event(instance,
		"Normal",
		"Delete RoleBinding",
		fmt.Sprintf("RoleBinding %s deleted", roleRuleFound.Name))

	return nil

}

func (r *ProjectSetReconciler) checkAndDeleteRoleRule(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace,
	name string) error {

	// Find if limitrange exists
	roleRuleFound := &rbacv1.Role{}

	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace.Name}, roleRuleFound)

	if err != nil && apierrors.IsNotFound(err) {

		return nil

	} else if err != nil {

		log.Error(err, "Failed to get RoleRule")
		// Reconcile failed due to error - requeue
		return err
	}

	// delete rule because not found in CR
	err = r.Delete(ctx, roleRuleFound)

	if err != nil {
		log.Error(err, "Failed to delete RoleRule", "Namespace", roleRuleFound.Namespace, "Name", roleRuleFound.Name)
		return err
	}

	// Save event
	r.Recorder.Event(instance,
		"Normal",
		"Delete RoleRule",
		fmt.Sprintf("RoleRule %s deleted", roleRuleFound.Name))

	return nil

}

// Role Binding logic create/update
// Return
//   - nil, nil  nothing's changed
//   - nil, err  error occured
//   - obj, nil  created/updated object
func (r *ProjectSetReconciler) createAndUpdateRoleBindings(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace) (*rbacv1.RoleBinding, error) {

	//check if defined in instance
	if instance.Spec.RoleBindings == nil {
		log.Info("RoleBindings is not defined")
		//return nil, nil
	}

	// Find if exists
	roleBindFound := &rbacv1.RoleBinding{}

	// Get existing list of bindings

	existingRoleBindNames, _ := r.roleBindNamesForNamespace(namespace.Name)

	log.Info("existingRulesNames", "names", existingRoleBindNames)

	// iterate thru rules policies
	for groupName, roleNames := range instance.Spec.RoleBindings {

		log.Info("Working on role bind", "name", groupName)

		err := r.Get(ctx, types.NamespacedName{Name: groupName, Namespace: namespace.Name}, roleBindFound)

		if err != nil && apierrors.IsNotFound(err) {

			// define a new network policy
			lr, err := r.roleBindForNamespace(instance, namespace, groupName, roleNames)

			if err != nil {
				log.Error(err, "Failed to define new RoleBinding")
				return nil, err
			}

			log.Info("Creating a new RoleBinding", "Name", lr.Name)

			err = r.Create(ctx, lr)

			if err != nil {
				log.Error(err, "Failed to create new RoleBinding", "Namespace", lr.Namespace, "Name", lr.Name)
				return nil, err
			}

			// Save event
			r.Recorder.Event(instance,
				"Normal",
				"Create RoleBinding",
				fmt.Sprintf("RoleBinding %s created", lr.Name))

			// role policy created, return and requeue
			return lr, nil

		} else if err != nil {

			log.Error(err, "Failed to get RoleBinding")
			// Reconcile failed due to error - requeue
			return nil, err
		}

		// Check if it changed
		if err := r.checkAndUpdateRoleBind(ctx, req, instance, namespace, groupName, roleBindFound); err != nil {
			return nil, err
		}

		delete(existingRoleBindNames, groupName)

	}

	// iterate thru old and delete

	for k, _ := range existingRoleBindNames {

		log.Info("Delete old role binding", "name", k)

		if err := r.checkAndDeleteRoleBind(ctx, req, instance, namespace, k); err != nil {
			return nil, err
		}
	}

	return nil, nil

}

// Role Rule logic create/update
// Return
//   - nil, nil  nothing's changed
//   - nil, err  error occured
//   - obj, nil  created/updated object
func (r *ProjectSetReconciler) createAndUpdateRoles(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace) (*rbacv1.Role, error) {

	//check if defined in instance
	if instance.Spec.Roles == nil {
		log.Info("Roles is not defined")
		//return nil, nil
	}

	// Find if limitrange exists
	roleRuleFound := &rbacv1.Role{}

	// Get existing list of policies

	existingRulesNames, _ := r.rolesNamesForNamespace(namespace.Name)

	log.Info("existingRulesNames", "names", existingRulesNames)

	// iterate thru rules policies
	for ruleName, ruleSpec := range instance.Spec.Roles {

		log.Info("Working on role rules", "name", ruleName)

		err := r.Get(ctx, types.NamespacedName{Name: ruleName, Namespace: namespace.Name}, roleRuleFound)

		if err != nil && apierrors.IsNotFound(err) {

			// define a new network policy
			lr, err := r.roleRuleForNamespace(instance, namespace, ruleName, ruleSpec)

			if err != nil {
				log.Error(err, "Failed to define new RoleRule")
				return nil, err
			}

			log.Info("Creating a new RoleRule", "Name", lr.Name)

			err = r.Create(ctx, lr)

			if err != nil {
				log.Error(err, "Failed to create new RoleRule", "Namespace", lr.Namespace, "Name", lr.Name)
				return nil, err
			}

			// Save event
			r.Recorder.Event(instance,
				"Normal",
				"Create RoleRule",
				fmt.Sprintf("RoleRule %s created", lr.Name))

			// role policy created, return and requeue
			return lr, nil

		} else if err != nil {

			log.Error(err, "Failed to get RoleRule")
			// Reconcile failed due to error - requeue
			return nil, err
		}

		// Check if it changed
		// Check resource quota is chnaged
		if err := r.checkAndUpdateRoleRule(ctx, req, instance, namespace, ruleName, roleRuleFound); err != nil {
			return nil, err
		}

		delete(existingRulesNames, ruleName)

	}

	// iterate thru old network and delete

	for k, _ := range existingRulesNames {

		log.Info("Delete old rule role", "name", k)

		if err := r.checkAndDeleteRoleRule(ctx, req, instance, namespace, k); err != nil {
			return nil, err
		}
	}

	return nil, nil

}

func (r *ProjectSetReconciler) checkAndDeleteNetworkPolicy(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace,
	name string) error {

	// Find if limitrange exists
	networkPolicyFound := &networkingv1.NetworkPolicy{}

	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace.Name}, networkPolicyFound)

	if err != nil && apierrors.IsNotFound(err) {

		return nil

	} else if err != nil {

		log.Error(err, "Failed to get NetworkPolicy")
		// Reconcile failed due to error - requeue
		return err
	}

	// delete policy because not found in CR
	err = r.Delete(ctx, networkPolicyFound)

	if err != nil {
		log.Error(err, "Failed to delete  NetworkPolicy", "Namespace", networkPolicyFound.Namespace, "Name", networkPolicyFound.Name)
		return err
	}

	// Save event
	r.Recorder.Event(instance,
		"Normal",
		"Delete NetworkPolicy",
		fmt.Sprintf("NetworkPolicy %s deleted", networkPolicyFound.Name))

	return nil

}

// Network Policy logic create/update
// Return
//   - nil, nil  nothing's changed
//   - nil, err  error occured
//   - obj, nil  created/updated object
func (r *ProjectSetReconciler) createAndUpdateNetworkPolicies(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace) (*networkingv1.NetworkPolicy, error) {

	//isDelete := false

	//check if defined in instance
	if instance.Spec.NetworkPolicy == nil {
		log.Info("NetworkPolicy is not defined")
		//return nil, nil
		//isDelete = true
	}

	// Find if limitrange exists
	networkPolicyFound := &networkingv1.NetworkPolicy{}

	// Get existing list of policies

	existingPolicyNames, _ := r.networkPolicyNamesForNamespace(namespace.Name)

	log.Info("existingPolicyNames", "names", existingPolicyNames)

	// iterate thru network policies
	for netName, netSpec := range instance.Spec.NetworkPolicy {

		log.Info("Working on network policy", "name", netName)

		err := r.Get(ctx, types.NamespacedName{Name: netName, Namespace: namespace.Name}, networkPolicyFound)

		if err != nil && apierrors.IsNotFound(err) {

			// define a new network policy
			lr, err := r.networkPolicyForNamespace(instance, namespace, netName, netSpec)

			if err != nil {
				log.Error(err, "Failed to define new NetworkPolicy")
				return nil, err
			}

			log.Info("Creating a new NetworkPolicy", "Name", lr.Name)

			err = r.Create(ctx, lr)

			if err != nil {
				log.Error(err, "Failed to create new NetworkPolicy", "Namespace", lr.Namespace, "Name", lr.Name)
				return nil, err
			}

			// Save event
			r.Recorder.Event(instance,
				"Normal",
				"Create NetworkPolicy",
				fmt.Sprintf("NetworkPolicy %s created", lr.Name))

			// network policy created, return and requeue
			return lr, nil

		} else if err != nil {

			log.Error(err, "Failed to get NetworkPolicy")
			// Reconcile failed due to error - requeue
			return nil, err
		}

		// Check if it changed
		// Check resource quota is chnaged
		if err := r.checkAndUpdateNetworkPolicy(ctx, req, instance, namespace, netName, networkPolicyFound); err != nil {
			return nil, err
		}

		delete(existingPolicyNames, netName)

	}

	// iterate thru old network and delete

	for k, _ := range existingPolicyNames {

		log.Info("Delete old networkpolicy", "name", k)

		if err := r.checkAndDeleteNetworkPolicy(ctx, req, instance, namespace, k); err != nil {
			return nil, err
		}
	}

	return nil, nil

}

// Limit Range logic create/update
// Return
//   - nil, nil  nothing's changed
//   - nil, err  error occured
//   - obj, nil  created/updated object
func (r *ProjectSetReconciler) createAndUpdateLimitRange(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace) (*corev1.LimitRange, error) {

	isDelete := false

	//check if defined in instance
	if len(instance.Spec.LimitRange.Limits) == 0 {
		log.Info("LimitRange is not defined")

		isDelete = true
		//return nil, nil
	}

	// Find if limitrange exists
	limitRangeFound := &corev1.LimitRange{}
	err := r.Get(ctx, types.NamespacedName{Name: namespace.Name, Namespace: namespace.Name}, limitRangeFound)

	if err != nil && apierrors.IsNotFound(err) && !isDelete {

		// define a new limit range
		lr, err := r.limitRangeForNamespace(instance, namespace, instance)

		if err != nil {
			log.Error(err, "Failed to define new ResourceQuota")
			return nil, err
		}

		log.Info("Creating a new LimitRange", "Name", lr.Name)

		err = r.Create(ctx, lr)

		if err != nil {
			log.Error(err, "Failed to create new LimitRange", "Namespace", lr.Namespace, "Name", lr.Name)
			return nil, err
		}

		// Save event
		r.Recorder.Event(instance,
			"Normal",
			"Create LimitRange",
			fmt.Sprintf("LimitRange %s created", lr.Name))

		// limitrange created, return and requeue
		return lr, nil

	} else if err != nil && apierrors.IsNotFound(err) && isDelete {

		return nil, nil

	} else if err != nil {

		log.Error(err, "Failed to get LimitRange")
		// Reconcile failed due to error - requeue
		return nil, err
	}

	log.Info("LimitRange exists")

	if isDelete {

		// delete rule because not found in CR
		err = r.Delete(ctx, limitRangeFound)
		if err != nil {
			return nil, err
		}

		// Save event
		r.Recorder.Event(instance,
			"Normal",
			"Delete LimitRange",
			fmt.Sprintf("LimitRange %s deleted", limitRangeFound.GetName()))

	} else {

		// Check if it changed
		// Check resource quota is chnaged
		if err := r.checkAndUpdateLimitRange(ctx, req, instance, namespace, limitRangeFound); err != nil {
			return nil, err
		}
	}

	return nil, nil

}

// Resource Quota logic create/update
// Return
//   - nil, nil  nothing's changed
//   - nil, err  error occured
//   - obj, nil  created/updated object
func (r *ProjectSetReconciler) createAndUpdateResourceQuota(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace) (*corev1.ResourceQuota, error) {

	isDelete := false

	//check if defined in instance
	if instance.Spec.ResourceQuota.Hard == nil {
		log.Info("Resource quota is not defined")
		//return nil, nil
		isDelete = true
	}

	// Find if resourcequota exists
	resourceQuotaFound := &corev1.ResourceQuota{}
	err := r.Get(ctx, types.NamespacedName{Name: namespace.Name, Namespace: namespace.Name}, resourceQuotaFound)

	if err != nil && apierrors.IsNotFound(err) && !isDelete {

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

	} else if err != nil && apierrors.IsNotFound(err) && isDelete {

		return nil, nil

	} else if err != nil {

		log.Error(err, "Failed to get ResourceQuota")
		// Reconcile failed due to error - requeue
		return nil, err
	}

	log.Info("ResourceQuota exists")

	if isDelete {

		// delete rule because not found in CR
		err = r.Delete(ctx, resourceQuotaFound)
		if err != nil {
			return nil, err
		}

		// Save event
		r.Recorder.Event(instance,
			"Normal",
			"Delete ResourceQuota",
			fmt.Sprintf("ResourceQuota %s deleted", resourceQuotaFound.GetName()))

	} else {

		// Check if it changed
		// Check resource quota is chnaged
		if err := r.checkAndUpdateResourceQuota(ctx, req, instance, namespace, resourceQuotaFound); err != nil {
			return nil, err
		}

	}

	return nil, nil

}

// Get default limits limitRange
// hack to avoid validation issue
func getOrDefaultLimitRange(instance *projectv1alpha1.ProjectSet) []corev1.LimitRangeItem {

	log.Info("Ckeck limits in LimitRange and set")

	if instance.Spec.LimitRange.Limits == nil || len(instance.Spec.LimitRange.Limits) == 0 {

		limits := []corev1.LimitRangeItem{}

		log.Info("LimitRange is not defined. Create stub")

		return limits

	}

	return instance.Spec.LimitRange.Limits

}

func (r *ProjectSetReconciler) roleBindNamesForNamespace(namespace string) (map[string]bool, error) {

	rulePolicyNames := make(map[string]bool)

	// Retrieve network policies in the given namespace
	rulePolicyList := &rbacv1.RoleBindingList{}
	err := r.List(context.TODO(), rulePolicyList, &client.ListOptions{
		Namespace: namespace,
	})
	if err != nil {
		return nil, err
	}

	for _, np := range rulePolicyList.Items {
		log.Info("RoleRule Policy", "name", np.Name)
		rulePolicyNames[np.Name] = true
	}

	return rulePolicyNames, nil

}

func (r *ProjectSetReconciler) rolesNamesForNamespace(namespace string) (map[string]bool, error) {

	rulePolicyNames := make(map[string]bool)

	// Retrieve network policies in the given namespace
	rulePolicyList := &rbacv1.RoleList{}
	err := r.List(context.TODO(), rulePolicyList, &client.ListOptions{
		Namespace: namespace,
	})
	if err != nil {
		return nil, err
	}

	for _, np := range rulePolicyList.Items {
		log.Info("RoleRule Policy", "name", np.Name)
		rulePolicyNames[np.Name] = true
	}

	return rulePolicyNames, nil

}

// RoleBind Policy build logic
func (r *ProjectSetReconciler) roleBindForNamespace(instance *projectv1alpha1.ProjectSet, namespace *corev1.Namespace, name string, spec []rbacv1.Subject) (*rbacv1.RoleBinding, error) {

	labels := namespace.GetLabels()
	annotations := namespace.GetAnnotations()

	roleRule := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace.GetName(),
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Subjects: spec,
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role", // or "ClusterRole" for cluster-wide role
			Name:     name,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	// Set the ownerRef for the Namespace
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(namespace, roleRule, r.Scheme); err != nil {
		return nil, err
	}

	return roleRule, nil
}

// Rule Policy build logic
func (r *ProjectSetReconciler) roleRuleForNamespace(instance *projectv1alpha1.ProjectSet, namespace *corev1.Namespace, name string, spec []rbacv1.PolicyRule) (*rbacv1.Role, error) {

	labels := namespace.GetLabels()
	annotations := namespace.GetAnnotations()

	roleRule := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace.GetName(),
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Rules: spec,
	}

	// Set the ownerRef for the Namespace
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(namespace, roleRule, r.Scheme); err != nil {
		return nil, err
	}

	return roleRule, nil
}

// Check Template with ProjectSet
func (r *ProjectSetReconciler) checkAndUpdateTemplate(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
) error {

	log.Info("checkAndUpdateTemplate", "name", instance.Spec.Template)

	//
	// check instance template and fill data if template exists
	//

	if &instance.Spec.Template != nil {

		log.Info("check template", "name", instance.Spec.Template)

		templateFound := &projectv1alpha1.ProjectSetTemplate{}

		err := r.Get(ctx, types.NamespacedName{Name: instance.Spec.Template}, templateFound)

		if err != nil && apierrors.IsNotFound(err) {
			return nil

		} else if err != nil {
			log.Error(err, "Failed to load template")
			return err
		}

		log.Info("Template exists", "template", templateFound)

		// template exist -> fill instance

		// labels
		labels := templateFound.Spec.Labels

		log.Info("Template labels", "labels", labels)

		log.Info("Instance labels", "labels", instance.Spec.Labels)

		for k, v := range instance.Spec.Labels {
			labels[k] = v
		}

		log.Info("Template mergered labels", "labels", labels)

		instance.Spec.Labels = labels

		// labels
		annotations := templateFound.Spec.Annotations

		for k, v := range instance.Spec.Annotations {
			annotations[k] = v
		}

		instance.Spec.Annotations = annotations

		// other fields if set in instance - use it
		// if not - use template

		if instance.Spec.ResourceQuota.Hard == nil {
			log.Info("ResourceQuota is nil, update from template")
			instance.Spec.ResourceQuota = templateFound.Spec.ResourceQuota
		}

		if len(instance.Spec.LimitRange.Limits) == 0 {
			log.Info("LimitRange is nil, update from template")
			instance.Spec.LimitRange = templateFound.Spec.LimitRange

			//override if template set limits to nil
			instance.Spec.LimitRange.Limits = getOrDefaultLimitRange(instance)
		}

		if instance.Spec.Roles == nil {
			log.Info("Roles is nil, update from template")
			instance.Spec.Roles = templateFound.Spec.Roles
		}

		if instance.Spec.RoleBindings == nil {
			log.Info("RoleBindings is nil, update from template")
			instance.Spec.RoleBindings = templateFound.Spec.RoleBindings
		}

		if instance.Spec.NetworkPolicy == nil {
			log.Info("NetworkPolicy is nil, update from template")
			instance.Spec.NetworkPolicy = templateFound.Spec.NetworkPolicy
		}

		log.Info("checkAndUpdateTemplate", "instance", instance.Spec)

	}

	return nil

}

// Check RoleRule changes with ProjectSet
func (r *ProjectSetReconciler) checkAndUpdateRoleBind(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace,
	name string,
	lr *rbacv1.RoleBinding,
) error {

	// equality.Semantic.DeepDerivative(desiredObj.Spec, runtimeObj.Spec)
	log.Info("checkAndUpdateRoleBind", "spec", instance.Spec.RoleBindings[name], "lr", lr.Subjects)

	if !equality.Semantic.DeepDerivative(instance.Spec.RoleBindings[name], lr.Subjects) {

		log.Info("RoleBinding is dirreferent - update from instance")

		lr.Subjects = instance.Spec.RoleBindings[name]

		if err := r.Update(ctx, lr); err != nil {
			return err
		}

		// Save event
		r.Recorder.Event(instance,
			"Normal",
			"Update RoleBinding",
			fmt.Sprintf("RoleBinding %s updated", lr.GetName()))

	}

	return nil

}

// Check RoleRule changes with ProjectSet
func (r *ProjectSetReconciler) checkAndUpdateRoleRule(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace,
	name string,
	lr *rbacv1.Role,
) error {

	// equality.Semantic.DeepDerivative(desiredObj.Spec, runtimeObj.Spec)
	log.Info("checkAndUpdateRole", "spec", instance.Spec.Roles[name], "lr", lr.Rules)

	if !equality.Semantic.DeepDerivative(instance.Spec.Roles[name], lr.Rules) {

		log.Info("RoleRule is dirreferent - update from instance")

		lr.Rules = instance.Spec.Roles[name]

		if err := r.Update(ctx, lr); err != nil {
			return err
		}

		// Save event
		r.Recorder.Event(instance,
			"Normal",
			"Update RoleRule",
			fmt.Sprintf("RoleRule %s updated", lr.GetName()))

	}

	return nil

}

func (r *ProjectSetReconciler) networkPolicyNamesForNamespace(namespace string) (map[string]bool, error) {

	netPolicyNames := make(map[string]bool)

	// Retrieve network policies in the given namespace
	networkPolicyList := &networkingv1.NetworkPolicyList{}
	err := r.List(context.TODO(), networkPolicyList, &client.ListOptions{
		Namespace: namespace,
	})
	if err != nil {
		return nil, err
	}

	for _, np := range networkPolicyList.Items {
		log.Info("Network Policy", "name", np.Name)
		netPolicyNames[np.Name] = true
	}

	return netPolicyNames, nil

}

// Netwoek Policy build logic
func (r *ProjectSetReconciler) networkPolicyForNamespace(instance *projectv1alpha1.ProjectSet, namespace *corev1.Namespace, name string, spec networkingv1.NetworkPolicySpec) (*networkingv1.NetworkPolicy, error) {

	labels := namespace.GetLabels()
	annotations := namespace.GetAnnotations()

	networkpolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace.GetName(),
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: spec,
	}

	// Set the ownerRef for the Namespace
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(namespace, networkpolicy, r.Scheme); err != nil {
		return nil, err
	}

	return networkpolicy, nil
}

// Check NetworkPolicy changes with ProjectSet
func (r *ProjectSetReconciler) checkAndUpdateNetworkPolicy(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace,
	name string,
	lr *networkingv1.NetworkPolicy,
) error {

	// equality.Semantic.DeepDerivative(desiredObj.Spec, runtimeObj.Spec)
	log.Info("checkAndUpdateNetworkPolicy", "spec", instance.Spec.NetworkPolicy[name], "lr", lr.Spec)

	if !equality.Semantic.DeepDerivative(instance.Spec.NetworkPolicy[name], lr.Spec) {

		log.Info("NetworkPolicy is dirreferent - update from instance")

		lr.Spec = instance.Spec.NetworkPolicy[name]

		if err := r.Update(ctx, lr); err != nil {
			return err
		}

		// Save event
		r.Recorder.Event(instance,
			"Normal",
			"Update NetworkPolicy",
			fmt.Sprintf("NetworkPolicy %s updated", lr.GetName()))

	}

	return nil

}

// Limit Range build logic
func (r *ProjectSetReconciler) limitRangeForNamespace(instance *projectv1alpha1.ProjectSet, namespace *corev1.Namespace, projSet *projectv1alpha1.ProjectSet) (*corev1.LimitRange, error) {

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
			Limits: getOrDefaultLimitRange(instance),
		},
	}

	// Set the ownerRef for the Namespace
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(namespace, limitRange, r.Scheme); err != nil {
		return nil, err
	}

	return limitRange, nil
}

// Check LimitRange changes with ProjectSet
func (r *ProjectSetReconciler) checkAndUpdateLimitRange(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace,
	lr *corev1.LimitRange,
) error {

	// equality.Semantic.DeepDerivative(desiredObj.Spec, runtimeObj.Spec)
	log.Info("checkAndUpdateLimitRange", "spec", instance.Spec.ResourceQuota, "lr", lr.Spec)

	if !equality.Semantic.DeepDerivative(instance.Spec.LimitRange, lr.Spec) {

		log.Info("LimitRange is dirreferent - update from instance")

		lr.Spec = instance.Spec.LimitRange

		if err := r.Update(ctx, lr); err != nil {
			return err
		}

		// Save event
		r.Recorder.Event(instance,
			"Normal",
			"Update LimitRange",
			fmt.Sprintf("LimitRange %s updated", lr.GetName()))

	}

	return nil

}

// Check ResourceQuota changes with ProjectSet
func (r *ProjectSetReconciler) checkAndUpdateResourceQuota(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSet,
	namespace *corev1.Namespace,
	rq *corev1.ResourceQuota,
) error {

	// if label or annotations changed - update namespace
	// equality.Semantic.DeepDerivative(desiredObj.Spec, runtimeObj.Spec)

	log.Info("checkAndUpdateResourceQuota", "spec", instance.Spec.ResourceQuota, "rq", rq.Spec)

	if !equality.Semantic.DeepDerivative(instance.Spec.ResourceQuota, rq.Spec) {

		log.Info("ResourceQuota is dirreferent - update from instance")

		rq.Spec = instance.Spec.ResourceQuota

		if err := r.Update(ctx, rq); err != nil {
			return err
		}

		// Save event
		r.Recorder.Event(instance,
			"Normal",
			"Update ResourceQuota",
			fmt.Sprintf("ResourceQuota %s updated", rq.GetName()))

	}

	return nil

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
	if !utils.IsMapSubset(instance.Spec.Labels, namespace.ObjectMeta.Labels) ||
		!utils.IsMapSubset(instance.Spec.Annotations, namespace.ObjectMeta.Annotations) {

		log.Info("Namespace labels or annotations are dirreferent - update from instance")

		namespace.ObjectMeta.Labels = instance.Spec.Labels
		namespace.ObjectMeta.Annotations = instance.Spec.Annotations

		if err := r.Update(ctx, namespace); err != nil {

			// Update status
			if err := r.setStatus(ctx, req, instance,
				typeAvailableStatus,
				metav1.ConditionFalse,
				"Reconciling",
				fmt.Sprintf("Failed to update namespace %s", namespace.Name)); err != nil {

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
	instance.Spec.LimitRange.Limits = getOrDefaultLimitRange(instance)

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

func (r *ProjectSetReconciler) findProjectSetByKeyValue(ctx context.Context, key string, value string) ([]projectv1alpha1.ProjectSet, error) {

	psList := &projectv1alpha1.ProjectSetList{}

	listPsFound := []projectv1alpha1.ProjectSet{}

	err := r.List(ctx, psList)

	if err != nil {

		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get ProjectSet")
		return nil, err
	}

	for _, ps := range psList.Items {

		if ps.Spec.Template == value {
			listPsFound = append(listPsFound, ps)
		}
	}

	return listPsFound, nil

}

func (r *ProjectSetReconciler) findProjectSetByName(ctx context.Context, name string) ([]projectv1alpha1.ProjectSet, error) {

	psFound := &projectv1alpha1.ProjectSet{}

	err := r.Get(ctx, types.NamespacedName{Name: name}, psFound)

	// if returned empty struct
	if name := psFound.GetName(); name == "" {
		return nil, fmt.Errorf("ProjectSet already deleted")
	}

	log.Info("findProjectSetByResourceQuotaName", "err", err, "name", name, "psFound", psFound)

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

			projSet, err := r.findProjectSetByName(ctx, rq.GetAnnotations()["projectset-name"])

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
		Watches(&corev1.LimitRange{
			TypeMeta: metav1.TypeMeta{
				Kind: "LimitRange",
			},
		}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}

			lr := a.(*corev1.LimitRange)

			projSet, err := r.findProjectSetByName(ctx, lr.GetAnnotations()["projectset-name"])

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
		Watches(&networkingv1.NetworkPolicy{
			TypeMeta: metav1.TypeMeta{
				Kind: "NetworkPolicy",
			},
		}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}

			lr := a.(*networkingv1.NetworkPolicy)

			projSet, err := r.findProjectSetByName(ctx, lr.GetAnnotations()["projectset-name"])

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
		Watches(&rbacv1.Role{
			TypeMeta: metav1.TypeMeta{
				Kind: "Role",
			},
		}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}

			lr := a.(*rbacv1.Role)

			projSet, err := r.findProjectSetByName(ctx, lr.GetAnnotations()["projectset-name"])

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
		Watches(&rbacv1.RoleBinding{
			TypeMeta: metav1.TypeMeta{
				Kind: "RoleBinding",
			},
		}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}

			lr := a.(*rbacv1.RoleBinding)

			projSet, err := r.findProjectSetByName(ctx, lr.GetAnnotations()["projectset-name"])

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
		Watches(&projectv1alpha1.ProjectSetTemplate{
			TypeMeta: metav1.TypeMeta{
				Kind: "ProjectSetTemplate",
			},
		}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}

			lr := a.(*projectv1alpha1.ProjectSetTemplate)

			projSet, err := r.findProjectSetByKeyValue(ctx, "Template", lr.GetName())

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

			return reconcileRequests
		})).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
