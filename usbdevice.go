// Inspired by https://github.com/kubevirt/kubevirt/tree/main/cmd/example-disk-mutation-hook-sidecar

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/clbanning/mxj"
	"google.golang.org/grpc"

	vmSchema "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/log"

	"kubevirt.io/kubevirt/pkg/hooks"
	hooksInfo "kubevirt.io/kubevirt/pkg/hooks/info"
	hooksV1alpha2 "kubevirt.io/kubevirt/pkg/hooks/v1alpha2"
	domainSchema "kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/api"
)

type infoServer struct{}

func (s infoServer) Info(ctx context.Context, params *hooksInfo.InfoParams) (*hooksInfo.InfoResult, error) {
	return &hooksInfo.InfoResult{
		Name: "usbdevice",
		Versions: []string{
			hooksV1alpha2.Version,
		},
		HookPoints: []*hooksInfo.HookPoint{
			{
				Name:     hooksInfo.OnDefineDomainHookPointName,
				Priority: 0,
			},
		},
	}, nil
}

type v1alpha2Server struct{}

func (s v1alpha2Server) OnDefineDomain(ctx context.Context, params *hooksV1alpha2.OnDefineDomainParams) (*hooksV1alpha2.OnDefineDomainResult, error) {
	log.Log.Info("OnDefineDomain hook callback method has been called")

	vmiJSON := params.GetVmi()
	vmiSpec := vmSchema.VirtualMachineInstance{}
	err := json.Unmarshal(vmiJSON, &vmiSpec)
	if err != nil {
		log.Log.Reason(err).Errorf("Failed to unmarshal given VMI spec: %s", vmiJSON)
		panic(err)
	}

	domainXML := params.GetDomainXML()
	domainSpec, err := mxj.NewMapXml(domainXML)
	if err != nil || domainSpec == nil {
		log.Log.Reason(err).Errorf("Failed to unmarshal given domain spec: %s", domainXML)
		panic(err)
	}

	hostdevSpec, err := domainSpec.ValuesForPath("domain.devices.hostdev")
	if err != nil {
		log.Log.Reason(err).Errorf("Failed to get values for path 'domain.devices.hostdev': %+v", domainSpec)
		panic(err)
	}

	annotations := vmiSpec.GetAnnotations()
	for key, value := range annotations {
		if strings.HasPrefix(key, "usbdevice.vm.kubevirt.io/") {
			alias := domainSchema.UserAliasPrefix + "usb-" + key[25:]

			var device mxj.Map

			if regexp.MustCompile("^[0-9a-fA-F]{4}:[0-9a-fA-F]{4}$").MatchString(value) {
				log.Log.Infof("Adding USB device '%s' targeted by 'vendor:product'='%s'", alias, value)

				vendorId := "0x" + value[:4]
				productId := "0x" + value[5:]

				device = mxj.Map{
					"alias": mxj.Map{
						"-name": alias,
					},
					"-type": "usb",
					"source": mxj.Map{
						"-startupPolicy": "optional",
						"vendor": mxj.Map{
							"-id": vendorId,
						},
						"product": mxj.Map{
							"-id": productId,
						},
					},
				}
			} else if regexp.MustCompile("^\\d{3}:\\d{3}$").MatchString(value) {
				log.Log.Infof("Adding USB device '%s' targeted by address 'bus:device'='%s'", alias, value)

				busId, _ := strconv.Atoi(value[:3])
				deviceId, _ := strconv.Atoi(value[4:])

				device = mxj.Map{
					"alias": mxj.Map{
						"-name": alias,
					},
					"-type": "usb",
					"source": mxj.Map{
						"-startupPolicy": "optional",
						"address": mxj.Map{
							"-bus":    busId,
							"-device": deviceId,
						},
					},
				}
			} else {
				err := fmt.Errorf("USB device selector '%s' does not match any of the expected formats", value)
				log.Log.Reason(err).Errorf("Failed to apply annotation: %s", key)
				panic(err)
			}

			hostdevSpec = append(hostdevSpec, device)
		}
	}

	_, err = domainSpec.UpdateValuesForPath(mxj.Map{
		"hostdev": hostdevSpec,
	}, "domain.devices.hostdev")
	if err != nil {
		log.Log.Reason(err).Errorf("Failed to update values for path 'domain.devices.hostdev': %+v", hostdevSpec)
		panic(err)
	}

	newDomainXML, err := domainSpec.Xml()
	if err != nil {
		log.Log.Reason(err).Errorf("Failed to marshal updated domain spec: %+v", domainSpec)
		panic(err)
	}

	log.Log.Info("Successfully updated original domain spec with requested boot disk attribute")
	return &hooksV1alpha2.OnDefineDomainResult{
		DomainXML: newDomainXML,
	}, nil
}

func (s v1alpha2Server) PreCloudInitIso(_ context.Context, params *hooksV1alpha2.PreCloudInitIsoParams) (*hooksV1alpha2.PreCloudInitIsoResult, error) {
	log.Log.Info("PreCloudInitIso hook callback method has been called")

	return &hooksV1alpha2.PreCloudInitIsoResult{
		CloudInitData: params.GetCloudInitData(),
	}, nil
}

func main() {
	log.InitializeLogging("usbdevice-hook-sidecar")

	socketPath := filepath.Join(hooks.HookSocketsSharedDirectory, "usbdevice.sock")
	socket, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Log.Reason(err).Errorf("Failed to initialized socket on path: %s", socket)
		log.Log.Error("Check whether given directory exists and socket name is not already taken by other file")
		panic(err)
	}
	defer os.Remove(socketPath)

	server := grpc.NewServer([]grpc.ServerOption{}...)
	hooksInfo.RegisterInfoServer(server, infoServer{})
	hooksV1alpha2.RegisterCallbacksServer(server, v1alpha2Server{})
	log.Log.Infof("Starting hook server exposing 'info' and 'v1alpha2' services on socket %s", socketPath)
	server.Serve(socket)
}
