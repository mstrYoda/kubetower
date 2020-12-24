package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type ClusterConnection struct {
	connections map[string]*kubernetes.Clientset
}

func NewClusterConnection() *ClusterConnection {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// TODO: handle error
	cfg, _ := clientcmd.LoadFromFile(*kubeconfig)

	clusterConnections := make(map[string]*kubernetes.Clientset)

	for _, context := range cfg.Contexts {
		// TODO: respect to name differences between contexts-clusters
		cc := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{CurrentContext: context.Cluster})

		restConfig, err := cc.ClientConfig()
		if err != nil {
			panic(err.Error())
		}

		clientSet, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			panic(err.Error())
		}

		clusterConnections[context.Cluster] = clientSet
	}

	return &ClusterConnection{
		connections: clusterConnections,
	}
}

func (c ClusterConnection) GetDeployments(clusters []string) (map[string][]v1.Deployment, []error) {
	deploymentClusterMap := make(map[string][]v1.Deployment)

	//TODO return correct errors for each cluster
	var errors []error

	for _, cluster := range clusters {
		deployments, err := c.connections[cluster].AppsV1().Deployments("").List(metav1.ListOptions{})
		if err != nil {
			errors = append(errors, err)
		}

		deploymentClusterMap[cluster] = deployments.Items
	}

	return deploymentClusterMap, errors
}

var clusterConnection *ClusterConnection

func (c ClusterConnection) RolloutRestartDeployment(deploymentName, namespace string, clusters []string) []error {
	timeNow := time.Now().UTC().Format(time.RFC3339)
	restartRequest := "{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{\"kubectl.kubernetes.io/restartedAt\":\"" + timeNow + "\"}}}}}"

	var errs []error

	for _, cluster := range clusters {
		_, err := c.connections[cluster].AppsV1().Deployments(namespace).Patch(deploymentName, types.StrategicMergePatchType, []byte(restartRequest))

		if err != nil {
			errs = append(errs, err)
		} else {
			errs = append(errs, nil)
		}
	}

	return errs
}

func (c ClusterConnection) ScaleDeployment(deploymentName, namespace string, clusters []string, replica int32) []error {

	var errs []error

	for _, cluster := range clusters {
		scale, err := c.connections[cluster].AppsV1().Deployments(namespace).GetScale(deploymentName, metav1.GetOptions{})
		if err != nil {
			errs = append(errs, err)
		}

		newScale := *scale
		newScale.Spec.Replicas = replica

		_, err = c.connections[cluster].AppsV1().Deployments(namespace).UpdateScale(deploymentName, &newScale)

		if err != nil {
			errs = append(errs, err)
		} else {
			errs = append(errs, nil)
		}
	}

	return errs
}

func main() {
	clusterConnection = NewClusterConnection()

	//PUT clusters/add
	//GET clusters/

	//GET resources/deployments?clusters=
	//PUT resources/deployments

	router := httprouter.New()
	router.GET("/resources/deployments", GetDeployments)
	router.POST("/resources/deployments/restart", RolloutRestartDeployment)
	router.POST("/resources/deployments/scale", ScaleDeployment)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}

type RolloutRestartDeploymentRequest struct {
	DeploymentName string   `json:"deploymentName"`
	Namespace      string   `json:"namespace"`
	Clusters       []string `json:"clusters"`
}

type ScaleDeploymentRequest struct {
	DeploymentName string   `json:"deploymentName"`
	Namespace      string   `json:"namespace"`
	Clusters       []string `json:"clusters"`
	Replicas       int32    `json:"replicas"`
}

func RolloutRestartDeployment(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var request RolloutRestartDeploymentRequest
	reqBytes, err := ioutil.ReadAll(r.Body)

	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(reqBytes, &request)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	errs := clusterConnection.RolloutRestartDeployment(request.DeploymentName, request.Namespace, request.Clusters)

	responseByte, _ := json.Marshal(errs)
	w.Write(responseByte)
}

func ScaleDeployment(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var request ScaleDeploymentRequest
	reqBytes, err := ioutil.ReadAll(r.Body)

	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(reqBytes, &request)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	errs := clusterConnection.ScaleDeployment(request.DeploymentName, request.Namespace, request.Clusters, request.Replicas)

	responseByte, _ := json.Marshal(errs)
	w.Write(responseByte)
}

func GetDeployments(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	clusters := r.URL.Query().Get("clusters")

	deployments, _ := clusterConnection.GetDeployments(strings.Split(clusters, ","))

	responseBytes, _ := json.Marshal(deployments)

	w.Write(responseBytes)
	w.WriteHeader(http.StatusOK)
}
