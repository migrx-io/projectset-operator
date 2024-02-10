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
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logger "sigs.k8s.io/controller-runtime/pkg/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	projectv1alpha1 "github.com/migrx-io/projectset-operator/api/v1alpha1"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Finilizer name
const projectSetSyncFinalizer = "projectsetssync.project.migrx.io/finalizer"

const ERR_TIMEOUT = 10

// ProjectSetSyncReconciler reconciles a ProjectSetSync object
type ProjectSetSyncReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsetsyncs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsetsyncs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsetsyncs/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ProjectSetSync object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *ProjectSetSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log = logger.FromContext(ctx).WithValues("projectsetsync", req.NamespacedName)

	//
	// Fetch the instance
	//
	instance := &projectv1alpha1.ProjectSetSync{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("ProjectSetSync resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get ProjectSetSync")
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
	// Git fetch Logic
	//
	log.Info("Fetch data from git",
		"GitRepo", instance.Spec.GitRepo,
		"EnvName", instance.Spec.EnvName,
		"GitBranch", instance.Spec.GitBranch,
		"GitSecretName", instance.Spec.GitSecretName,
		"SyncSecInterval", instance.Spec.SyncSecInterval,
		"ConfFile", instance.Spec.ConfFile,
	)

	localDir := "/tmp/" + instance.Spec.EnvName

	token := os.Getenv("GIT_TOKEN")

	_, err = git.PlainClone(localDir, false, &git.CloneOptions{
		Auth: &http.BasicAuth{
			Username: "projectsetsync",
			Password: token,
		},
		URL:      instance.Spec.GitRepo,
		Progress: os.Stdout,
	})

	if err != nil {
		log.Error(err, "Error cloning repository")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	// Open the repository
	repo, err := git.PlainOpen(localDir)
	if err != nil {
		log.Error(err, "Error opening repository")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	// Checkout to the desired branch
	worktree, err := repo.Worktree()
	if err != nil {
		log.Error(err, "Error geting worktree")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(instance.Spec.GitBranch),
	})
	if err != nil {

		log.Error(err, "Error checking out branch")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	// sleep and check again
	return ctrl.Result{Requeue: true,
		RequeueAfter: time.Duration(instance.Spec.SyncSecInterval) * time.Second}, nil

}

// Finalizers will perform the required operations before delete the CR.
func (r *ProjectSetSyncReconciler) doFinalizerOperations(cr *projectv1alpha1.ProjectSetSync) {
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

func (r *ProjectSetSyncReconciler) setStatus(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSetSync,
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

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectSetSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projectv1alpha1.ProjectSetSync{}).
		Complete(r)
}
