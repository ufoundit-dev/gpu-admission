package route

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"k8s.io/klog/v2"
	log "k8s.io/klog/v2"

	"github.com/julienschmidt/httprouter"
	extender "k8s.io/kube-scheduler/extender/v1"
)

const (
	bindPrefixV2       = apiPrefix + "/bind"
	predicatesPrefixV2 = apiPrefix + "/filter"
	prioritiesPrefixV2 = apiPrefix + "/priorities"
	statusPrefixV2     = apiPrefix + "/status"
)

func AddVersionV2(router *httprouter.Router) {
	router.GET(versionPath, DebugLogging(VersionRoute, versionPath))
}

func PredicateRouteV2() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var buf bytes.Buffer
		body := io.TeeReader(r.Body, &buf)

		var extenderArgs extender.ExtenderArgs
		var extenderFilterResult *extender.ExtenderFilterResult

		if err := json.NewDecoder(body).Decode(&extenderArgs); err != nil {

			log.Warning("Failed to parse request due to error: %v", err)
			extenderFilterResult = &extender.ExtenderFilterResult{
				Nodes:       nil,
				FailedNodes: nil,
				Error:       err.Error(),
			}
		} else {
			log.V(5).Infof("GpuSharingFilter ExtenderArgs: %+v", extenderArgs)
			if extenderArgs.NodeNames == nil {
				extenderFilterResult = &extender.ExtenderFilterResult{
					Nodes:       nil,
					FailedNodes: nil,
					Error:       "gpu-admission extender must be configured with nodeCacheCapable=true",
				}
			} else {
				log.Infof("Start to filter for pod %s/%s", extenderArgs.Pod.Namespace, extenderArgs.Pod.Name)
				var nodesName []string
				for _, v := range *extenderArgs.NodeNames {
					nodesName = append(nodesName, v)
				}
				extenderFilterResult = &extender.ExtenderFilterResult{
					NodeNames:   &nodesName,
					FailedNodes: extender.FailedNodesMap{},
					Error:       "",
				}
			}
		}

		if resultBody, err := json.Marshal(extenderFilterResult); err != nil {
			log.Warningf("Failed to parse filter result: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			errMsg := fmt.Sprintf("{'error':'%s'}", err.Error())
			w.Write([]byte(errMsg))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(resultBody)
		}
	}
}

func AddPredicateV2(router *httprouter.Router) {
	router.POST(predicatesPrefixV2, PredicateRouteV2())
}

func PrioritizeRouteV2() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var buf bytes.Buffer
		body := io.TeeReader(r.Body, &buf)
		log.V(5).Info("priority ExtenderArgs = ", buf.String())

		var extenderArgs extender.ExtenderArgs
		var hostPriorityList *extender.HostPriorityList

		if err := json.NewDecoder(body).Decode(&extenderArgs); err != nil {
			panic(err)
		}

		log.Infof("Start to score for pod %s/%s", extenderArgs.Pod.Namespace, extenderArgs.Pod.Name)
		list := make(extender.HostPriorityList, len(*extenderArgs.NodeNames))
		for i, v := range *extenderArgs.NodeNames {
			list[i] = extender.HostPriority{
				Host:  v,
				Score: int64(100),
			}
		}

		hostPriorityList = &list

		if resultBody, err := json.Marshal(hostPriorityList); err != nil {
			panic(err)
		} else {
			log.Info("%s hostPriorityList: %s", "gpumanager", string(resultBody))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(resultBody)
		}
	}
}

func AddPrioritizeV2(router *httprouter.Router) {
	router.POST(prioritiesPrefixV2, PrioritizeRouteV2())
}

func BindRouteV2() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		checkBody(w, r)

		var buf bytes.Buffer
		body := io.TeeReader(r.Body, &buf)

		failed := false
		var extenderBindingArgs extender.ExtenderBindingArgs
		var extenderBindingResult = &extender.ExtenderBindingResult{
			Error: "",
		}
		if err := json.NewDecoder(body).Decode(&extenderBindingArgs); err != nil {
			extenderBindingResult = &extender.ExtenderBindingResult{
				Error: err.Error(),
			}
			failed = true
		} else {
			log.Infof("Start to bind pod %s/%s to node %s", extenderBindingArgs.PodNamespace, extenderBindingArgs.PodName, extenderBindingArgs.Node)
			log.V(5).Info("GpuSharingBind ExtenderArgs =", extenderBindingArgs)
		}

		klog.Infof("bind args +%v", extenderBindingArgs)
		if resultBody, err := json.Marshal(extenderBindingResult); err != nil {
			log.Warning("Fail to parse bind result: %+v", err)
			// panic(err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			errMsg := fmt.Sprintf("{'error':'%s'}", err.Error())
			w.Write([]byte(errMsg))
		} else {
			log.Info("extenderBindingResult = ", string(resultBody))
			w.Header().Set("Content-Type", "application/json")
			if failed {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
			}

			w.Write(resultBody)
		}
	}
}

func AddBindV2(router *httprouter.Router) {
	if handle, _, _ := router.Lookup("POST", bindPrefixV2); handle != nil {
		log.Warning("AddBind was called more then once")
	} else {
		router.POST(bindPrefixV2, BindRouteV2())
	}
}

func AddStatusV2(router *httprouter.Router) {
	router.GET(statusPrefixV2, func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		resultBody := []byte("{}")
		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK)
		w.Write(resultBody)
	})
}
