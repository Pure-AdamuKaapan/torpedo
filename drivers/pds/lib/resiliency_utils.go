package lib

import (
	"errors"
	"fmt"
	dataservices "github.com/portworx/torpedo/drivers/pds/dataservice"
	"math/rand"
	"strings"
	"sync"
	"time"

	pds "github.com/portworx/pds-api-go-client/pds/v1alpha1"
	"github.com/portworx/torpedo/drivers/node"

	_ "github.com/portworx/torpedo/drivers/scheduler/dcos"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/portworx/torpedo/pkg/log"
	"github.com/portworx/torpedo/tests"
)

const (
	PdsDeploymentControllerManagerPod = "pds-deployment-controller-manager"
	PdsAgentPod                       = "pds-agent"
	PdsTeleportPod                    = "pds-teleport"
	ActiveNodeRebootDuringDeployment  = "active-node-reboot-during-deployment"
	KillDeploymentControllerPod       = "kill-deployment-controller-pod-during-deployment"
	RestartPxDuringDSScaleUp          = "restart-portworx-during-ds-scaleup"
	RebootNodesDuringDeployment       = "reboot-multiple-nodes-during-deployment"
	KillAgentPodDuringDeployment      = "kill-agent-pod-during-deployment"
	RestartAppDuringResourceUpdate    = "restart-app-during-resource-update"
	UpdateTemplate                    = "medium"
	RebootNodeDuringAppVersionUpdate  = "reboot-node-during-app-version-update"
	KillTeleportPodDuringDeployment   = "kill-teleport-pod-during-deployment"
)

// PDS vars
var (
	dataservice               *dataservices.DataserviceType
	wg                        sync.WaitGroup
	ResiliencyFlag            = false
	hasResiliencyConditionMet = false
	FailureType               TypeOfFailure
	CapturedErrors            = make(chan error, 10)
	checkTillReplica          int32
	ResiliencyCondition       = make(chan bool)
)

// Struct Definition for kind of Failure the framework needs to trigger
type TypeOfFailure struct {
	Type   string
	Method func() error
}

// Wrapper to Define failure type from Test Case
func DefineFailureType(failuretype TypeOfFailure) {
	FailureType = failuretype
}

// Executes all methods in parallel
func ExecuteInParallel(functions ...func()) {
	wg.Add(len(functions))
	defer wg.Wait()
	for _, fn := range functions {
		go func(FuncToRun func()) {
			defer wg.Done()
			FuncToRun()
		}(fn)
	}
}

// Function to enable Resiliency Test
func MarkResiliencyTC(resiliency bool) {
	ResiliencyFlag = resiliency
}

// Function to wait for event to induce failure
func InduceFailure(failure string, ns string) {
	isResiliencyConditionset := <-ResiliencyCondition
	if isResiliencyConditionset {
		FailureType.Method()
	} else {
		CapturedErrors <- errors.New("Resiliency Condition did not meet. Failing this test case.")
		return
	}
	return
}

// Close all open Resiliency channels here
func CloseResiliencyChannel() {
	// Close the Channel if it's empty. Otherwise there is no need to close as per Golang official documentation,
	// as far as we are making sure no writes are happening to a closed channel. Make sure to call this method only
	// during Post Test Case execution to avoid any unknown panics
	if len(ResiliencyCondition) == 0 {
		close(ResiliencyCondition)
	}
}

func InduceFailureAfterWaitingForCondition(deployment *pds.ModelsDeployment, namespace string, CheckTillReplica int32) error {
	switch FailureType.Type {
	// Case when we want to reboot a node onto which a deployment pod is coming up
	case ActiveNodeRebootDuringDeployment:
		checkTillReplica = CheckTillReplica
		log.InfoD("Entering to check if Data service has %v active pods. Once it does, we will reboot the node it is hosted upon.", checkTillReplica)
		func1 := func() {
			GetPdsSs(deployment.GetClusterResourceName(), namespace, checkTillReplica)
		}
		func2 := func() {
			InduceFailure(FailureType.Type, namespace)
		}
		ExecuteInParallel(func1, func2)
	case RebootNodeDuringAppVersionUpdate:
		log.InfoD("Entering to check if Data service pods started update " +
			"Once it does, we restart portworx")
		func1 := func() {
			CheckPodIsTerminating(deployment.GetClusterResourceName(), namespace)
		}
		func2 := func() {
			InduceFailure(FailureType.Type, namespace)
		}
		ExecuteInParallel(func1, func2)
	case RestartPxDuringDSScaleUp:
		log.InfoD("Entering to check if Data service has %v active pods. "+
			"Once it does, we restart portworx", checkTillReplica)
		func1 := func() {
			GetPdsSs(deployment.GetClusterResourceName(), namespace, CheckTillReplica)
		}
		func2 := func() {
			InduceFailure(FailureType.Type, namespace)
		}
		ExecuteInParallel(func1, func2)
	case KillDeploymentControllerPod:
		checkTillReplica = CheckTillReplica
		log.InfoD("Entering to check if Data service has %v active pods. Once it does, we will kill the deployment Controller Pod.", checkTillReplica)
		func1 := func() {
			GetPdsSs(deployment.GetClusterResourceName(), namespace, checkTillReplica)
		}
		func2 := func() {
			InduceFailure(FailureType.Type, namespace)
		}
		ExecuteInParallel(func1, func2)
	case RestartAppDuringResourceUpdate:
		log.InfoD("Entering to check if Data service has %v active pods. "+
			"Once it does, we restart application pods", checkTillReplica)
		func1 := func() {
			UpdateDeploymentResourceConfig(deployment, namespace, UpdateTemplate)
		}
		func2 := func() {
			InduceFailure(FailureType.Type, namespace)
		}
		ExecuteInParallel(func1, func2)
	case RebootNodesDuringDeployment:
		checkTillReplica = CheckTillReplica
		log.InfoD("Entering to check if Data service has %v active pods. Once it does, we will start rebooting all worker nodes.", checkTillReplica)
		func1 := func() {
			GetPdsSs(deployment.GetClusterResourceName(), namespace, checkTillReplica)
		}
		func2 := func() {
			InduceFailure(FailureType.Type, namespace)
		}
		ExecuteInParallel(func1, func2)
	case KillAgentPodDuringDeployment:
		checkTillReplica = CheckTillReplica
		log.InfoD("Entering to check if Data service has %v active pods. Once it does, we will kill the Agent Pod.", checkTillReplica)
		func1 := func() {
			GetPdsSs(deployment.GetClusterResourceName(), namespace, checkTillReplica)
		}
		func2 := func() {
			InduceFailure(FailureType.Type, namespace)
		}
		ExecuteInParallel(func1, func2)
	case KillTeleportPodDuringDeployment:
		checkTillReplica = CheckTillReplica
		log.InfoD("Entering to check if Data service has %v active pods. Once it does, we will kill the Agent Pod.", checkTillReplica)
		func1 := func() {
			GetPdsSs(deployment.GetClusterResourceName(), namespace, checkTillReplica)
		}
		func2 := func() {
			InduceFailure(FailureType.Type, namespace)
		}
		ExecuteInParallel(func1, func2)
	}

	var aggregatedError error
	for w := 1; w <= len(CapturedErrors); w++ {
		if err := <-CapturedErrors; err != nil {
			aggregatedError = fmt.Errorf("%v : %v", aggregatedError, err)
		}
	}
	if aggregatedError != nil {
		return aggregatedError
	}
	//validate method needs to be called from the testcode
	err := dataservice.ValidateDataServiceDeployment(deployment, namespace)
	return err
}

func RestartPXDuringDSScaleUp(ns string, deployment *pds.ModelsDeployment) error {
	// Get StatefulSet Object
	var ss *v1.StatefulSet
	var testError error

	//Waiting till pod have a node assigned
	var pods []corev1.Pod
	var nodeToRestartPX node.Node
	var nodeName string
	var podName string
	err = wait.Poll(resiliencyInterval, timeOut, func() (bool, error) {
		ss, testError = k8sApps.GetStatefulSet(deployment.GetClusterResourceName(), ns)
		if testError != nil {
			CapturedErrors <- testError
			return false, testError
		}
		// Get Pods of this StatefulSet
		pods, testError = k8sApps.GetStatefulSetPods(ss)
		if testError != nil {
			CapturedErrors <- testError
			return false, testError
		}
		// Check if the new Pod have a node assigned or it's in a window where it's just coming up
		podCount := 0
		for _, pod := range pods {
			log.Infof("Nodename of pod %v is :%v:", pod.Name, pod.Spec.NodeName)
			if pod.Spec.NodeName == "" || pod.Spec.NodeName == " " {
				log.Infof("Pod %v still does not have a node assigned. Retrying in 5 seconds", pod.Name)
				return false, nil
			} else {
				podCount += 1
				log.Debugf("No of pods that has node assigned: %d", podCount)
			}
			if int32(podCount) == *ss.Spec.Replicas {
				log.Debugf("Expected pod %v has node %v assigned", pod.Name, pod.Spec.NodeName)
				nodeName = pod.Spec.NodeName
				podName = pod.Name
				return true, nil
			}
		}
		return true, nil
	})
	nodeToRestartPX, testError = node.GetNodeByName(nodeName)
	if testError != nil {
		CapturedErrors <- testError
		return testError
	}

	log.InfoD("Going ahead and restarting PX the node %v as there is an "+
		"application pod %v that's coming up on this node", nodeName, podName)
	testError = tests.Inst().V.RestartDriver(nodeToRestartPX, nil)
	if testError != nil {
		CapturedErrors <- testError
		return testError
	}

	log.InfoD("PX restarted successfully on node %v", podName)
	return testError
}

func NodeRebootDurinAppVersionUpdate(ns string, deployment *pds.ModelsDeployment) error {
	// Get StatefulSet Object
	var ss *v1.StatefulSet
	var testError error
	var nodeToReboot node.Node
	var nodeName, podName string

	// Waiting till atleast first pod have a node assigned
	var pods []corev1.Pod
	err = wait.PollImmediate(resiliencyInterval, timeOut, func() (bool, error) {
		ss, testError = k8sApps.GetStatefulSet(deployment.GetClusterResourceName(), ns)
		if testError != nil {
			CapturedErrors <- testError
			return false, testError
		}
		// Get Pods of this StatefulSet
		pods, testError = k8sApps.GetStatefulSetPods(ss)
		if testError != nil {
			CapturedErrors <- testError
			return false, testError
		}
		// Check if Pods have a node assigned or it's in a window where it's just coming up
		for _, pod := range pods {
			log.Infof("Nodename of pod %v is %v and deletiontimestamp is %v", pod.Name, pod.Spec.NodeName, pod.DeletionTimestamp)
			if pod.DeletionTimestamp != nil {
				podName = pod.Name
				nodeName = pod.Spec.NodeName
				return true, nil
			} else {
				return false, nil
			}
		}
		return true, nil
	})
	nodeToReboot, testError = node.GetNodeByName(nodeName)
	if testError != nil {
		CapturedErrors <- testError
		return testError
	}
	log.InfoD("Going ahead and restarting PX the node %v as there is an "+
		"application pod %v that's coming up on this node", nodeName, podName)

	testError = tests.Inst().N.RebootNode(nodeToReboot, node.RebootNodeOpts{
		Force: true,
		ConnectionOpts: node.ConnectionOpts{
			Timeout:         defaultCommandTimeout,
			TimeBeforeRetry: defaultCommandRetry,
		},
	})
	if testError != nil {
		CapturedErrors <- testError
		return testError
	}
	log.Infof("Node %v rebooted successfully", nodeName)
	return testError
}

// Reboot the Active Node onto which the application pod is coming up
func RebootActiveNodeDuringDeployment(ns string, deployment *pds.ModelsDeployment, num_reboots int) error {
	// Get StatefulSet Object
	var ss *v1.StatefulSet
	var testError error

	// Waiting till atleast first pod have a node assigned
	var pods []corev1.Pod
	err = wait.Poll(resiliencyInterval, timeOut, func() (bool, error) {
		ss, testError = k8sApps.GetStatefulSet(deployment.GetClusterResourceName(), ns)
		if testError != nil {
			CapturedErrors <- testError
			return false, testError
		}
		// Get Pods of this StatefulSet
		pods, testError = k8sApps.GetStatefulSetPods(ss)
		if testError != nil {
			CapturedErrors <- testError
			return false, testError
		}
		// Check if Pods have a node assigned or it's in a window where it's just coming up
		for _, pod := range pods {
			log.Infof("Nodename of pod %v is :%v:", pod.Name, pod.Spec.NodeName)
			if pod.Spec.NodeName == "" || pod.Spec.NodeName == " " {
				log.Infof("Pod %v still does not have a node assigned. Retrying in 5 seconds", pod.Name)
				return false, nil
			} else {
				return true, nil
			}
		}
		return true, nil
	})

	// Check which Pod is still not up. Try to reboot the node on which this Pod is hosted.
	for _, pod := range pods {
		log.Infof("Checking Pod %v running on Node: %v", pod.Name, pod.Spec.NodeName)
		if k8sCore.IsPodReady(pod) {
			log.InfoD("This Pod running on Node %v is Ready so skipping this pod......", pod.Spec.NodeName)
			continue
		} else {
			var nodeToReboot node.Node
			nodeToReboot, testError := node.GetNodeByName(pod.Spec.NodeName)
			if testError != nil {
				CapturedErrors <- testError
				return testError
			}
			if nodeToReboot.Name == "" {
				testError = errors.New("Something happened and node is coming out to be empty from Node registry")
				CapturedErrors <- testError
				return testError
			}
			log.Infof("Going ahead and rebooting the node %v as there is an application pod thats coming up on this node", pod.Spec.NodeName)
			testError = tests.Inst().N.RebootNode(nodeToReboot, node.RebootNodeOpts{
				Force: true,
				ConnectionOpts: node.ConnectionOpts{
					Timeout:         defaultCommandTimeout,
					TimeBeforeRetry: defaultCommandRetry,
				},
			})
			if testError != nil {
				CapturedErrors <- testError
				return testError
			}
			if num_reboots > 1 {
				for index := 1; index <= num_reboots; index++ {
					log.Infof("wait for node: %s to be back up", nodeToReboot.Name)
					err = tests.Inst().N.TestConnection(nodeToReboot, node.ConnectionOpts{
						Timeout:         defaultTestConnectionTimeout,
						TimeBeforeRetry: defaultWaitRebootRetry,
					})
					if err != nil {
						CapturedErrors <- err
						return err
					}
					testError = tests.Inst().N.RebootNode(nodeToReboot, node.RebootNodeOpts{
						Force: true,
						ConnectionOpts: node.ConnectionOpts{
							Timeout:         defaultCommandTimeout,
							TimeBeforeRetry: defaultCommandRetry,
						},
					})
					if testError != nil {
						CapturedErrors <- testError
						return testError
					}
				}
			}
			log.Infof("Node %v rebooted successfully", pod.Spec.NodeName)
		}
	}
	return testError
}

// Reboot All Worker Nodes while deployment is ongoing
func RebootWorkerNodesDuringDeployment(ns string, deployment *pds.ModelsDeployment, testType string) error {
	// Waiting till atleast first pod have a node assigned
	var pods []corev1.Pod
	err = wait.Poll(resiliencyInterval, timeOut, func() (bool, error) {
		ss, err := k8sApps.GetStatefulSet(deployment.GetClusterResourceName(), ns)
		if err != nil {
			CapturedErrors <- err
			return false, err
		}
		// Get Pods of this StatefulSet
		pods, err = k8sApps.GetStatefulSetPods(ss)
		if err != nil {
			CapturedErrors <- err
			return false, err
		}
		// Check if Pods have a node assigned or it's in a window where it's just coming up
		for _, pod := range pods {
			log.Infof("Nodename of pod %v is :%v:", pod.Name, pod.Spec.NodeName)
			if pod.Spec.NodeName == "" || pod.Spec.NodeName == " " {
				log.Infof("Pod %v still does not have a node assigned. Retrying in 5 seconds", pod.Name)
				return false, nil
			} else {
				return true, nil
			}
		}
		return true, nil
	})
	if err != nil {
		CapturedErrors <- err
		return err
	}
	// Reboot Worker Nodes depending on Test Type (all or quorum)
	nodesToReboot := node.GetWorkerNodes()
	if testType == "quorum" {
		num_nodes := len(nodesToReboot)
		if num_nodes < 3 {
			nodesToReboot = nodesToReboot[0:2]
		}
		quorum_nodes := (num_nodes / 2) + 1
		log.InfoD("Total number of nodes in Cluter: %v", num_nodes)
		log.InfoD("Rebooting %v nodes in Cluster", quorum_nodes)
		nodesToReboot = nodesToReboot[0:quorum_nodes]
	}

	for _, n := range nodesToReboot {
		log.InfoD("reboot node: %s", n.Name)
		err = tests.Inst().N.RebootNode(n, node.RebootNodeOpts{
			Force: true,
			ConnectionOpts: node.ConnectionOpts{
				Timeout:         defaultCommandTimeout,
				TimeBeforeRetry: defaultCommandRetry,
			},
		})
		if err != nil {
			CapturedErrors <- err
			return err
		}

		log.Infof("wait for node: %s to be back up", n.Name)
		err = tests.Inst().N.TestConnection(n, node.ConnectionOpts{
			Timeout:         defaultTestConnectionTimeout,
			TimeBeforeRetry: defaultWaitRebootRetry,
		})
		if err != nil {
			CapturedErrors <- err
			return err
		}
	}
	return nil
}

// Kill All pods matching podName string in a given namespace
func KillPodsInNamespace(ns string, podName string) error {
	var Pods []corev1.Pod
	// Fetch All the pods in pds-system namespace
	podList, testError := GetPods(ns)
	if testError != nil {
		CapturedErrors <- testError
		return testError
	}
	// Get List of All Pods matching with the name : podName
	for _, pod := range podList.Items {
		if strings.Contains(pod.Name, podName) {
			log.Infof("Pod Name is : %v", pod.Name)
			Pods = append(Pods, pod)
		}
	}
	// Kill All Pods matching with podName
	for _, pod := range Pods {
		log.InfoD("Deleting Pod: %s", pod.Name)
		testError = DeleteK8sPods(pod.Name, ns)
		if testError != nil {
			CapturedErrors <- testError
			return testError
		}
		log.InfoD("Successfully Killed Pod: %v", pod.Name)
	}
	return testError
}

func RestartApplicationDuringResourceUpdate(ns string, deployment *pds.ModelsDeployment) error {
	var ss *v1.StatefulSet
	ss, testError := k8sApps.GetStatefulSet(deployment.GetClusterResourceName(), ns)
	if testError != nil {
		CapturedErrors <- testError
		return testError
	}
	// Get Pods of this StatefulSet
	pods, testError := k8sApps.GetStatefulSetPods(ss)
	if testError != nil {
		CapturedErrors <- testError
		return testError
	}
	rand.Seed(time.Now().Unix())
	pod := pods[rand.Intn(len(pods))]
	// Delete the deployment Pods during update.
	testError = DeleteK8sPods(pod.Name, ns)
	if testError != nil {
		CapturedErrors <- testError
		return testError
	}
	return testError
}
