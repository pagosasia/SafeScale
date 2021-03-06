/*
 * Copyright 2018, CS Systemes d'Information, http://www.c-s.fr
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	server "github.com/CS-SI/SafeScale/broker/server/services"
	_ "github.com/CS-SI/SafeScale/broker/utils" // Imported to initialise tenants
	"github.com/CS-SI/SafeScale/providers"
	"github.com/CS-SI/SafeScale/providers/model"
	"github.com/CS-SI/SafeScale/providers/model/enums/IPVersion"
	"github.com/CS-SI/SafeScale/utils"
)

const cmdNumberOfCPU string = "lscpu | grep 'CPU(s):' | grep -v 'NUMA' | tr -d '[:space:]' | cut -d: -f2"
const cmdNumberOfCorePerSocket string = "lscpu | grep 'Core(s) per socket' | tr -d '[:space:]' | cut -d: -f2"
const cmdNumberOfSocket string = "lscpu | grep 'Socket(s)' | tr -d '[:space:]' | cut -d: -f2"
const cmdArch string = "lscpu | grep 'Architecture' | tr -d '[:space:]' | cut -d: -f2"
const cmdHypervisor string = "lscpu | grep 'Hypervisor' | tr -d '[:space:]' | cut -d: -f2"

const cmdCPUFreq string = "lscpu | grep 'CPU MHz' | tr -d '[:space:]' | cut -d: -f2"
const cmdCPUModelName string = "lscpu | grep 'Model name' | cut -d: -f2 | sed -e 's/^[[:space:]]*//'"
const cmdTotalRAM string = "cat /proc/meminfo | grep MemTotal | cut -d: -f2 | sed -e 's/^[[:space:]]*//' | cut -d' ' -f1"
const cmdRAMFreq string = "sudo dmidecode -t memory | grep Speed | head -1 | cut -d' ' -f2"

const cmdGPU string = "lspci | egrep -i 'VGA|3D' | grep -i nvidia | cut -d: -f3 | sed 's/.*controller://g' | tr '\n' '%'"
const cmdDiskSize string = "lsblk -b --output SIZE -n -d /dev/sda"
const cmdEphemeralDiskSize string = "lsblk -o name,type,mountpoint | grep disk | awk {'print $1'} | grep -v sda | xargs -i'{}' lsblk -b --output SIZE -n -d /dev/'{}'"
const cmdRotational string = "cat /sys/block/sda/queue/rotational"
const cmdDiskSpeed string = "sudo hdparm -t --direct /dev/sda | grep MB | awk '{print $11}'"
const cmdNetSpeed string = "URL=\"http://www.google.com\";curl -L --w \"$URL\nDNS %{time_namelookup}s conn %{time_connect}s time %{time_total}s\nSpeed %{speed_download}bps Size %{size_download}bytes\n\" -o/dev/null -s $URL | grep bps | awk '{ print $2}' | cut -d '.' -f 1"

var cmd = fmt.Sprintf("export LANG=C;echo $(%s)î$(%s)î$(%s)î$(%s)î$(%s)î$(%s)î$(%s)î$(%s)î$(%s)î$(%s)î$(%s)î$(%s)î$(%s)î$(%s)î$(%s)",
	cmdNumberOfCPU,
	cmdNumberOfCorePerSocket,
	cmdNumberOfSocket,
	cmdCPUFreq,
	cmdArch,
	cmdHypervisor,
	cmdCPUModelName,
	cmdTotalRAM,
	cmdRAMFreq,
	cmdGPU,
	cmdDiskSize,
	cmdEphemeralDiskSize,
	cmdDiskSpeed,
	cmdRotational,
	cmdNetSpeed,
)

//CPUInfo stores CPU properties
type CPUInfo struct {
	TenantName   string `json:"tenant_name,omitempty"`
	TemplateID   string `json:"template_id,omitempty"`
	TemplateName string `json:"template_name,omitempty"`
	ImageID      string `json:"image_id,omitempty"`
	ImageName    string `json:"image_name,omitempty"`
	LastUpdated  string `json:"last_updated,omitempty"`

	NumberOfCPU    int     `json:"number_of_cpu,omitempty"`
	NumberOfCore   int     `json:"number_of_core,omitempty"`
	NumberOfSocket int     `json:"number_of_socket,omitempty"`
	CPUFrequency   float64 `json:"cpu_frequency_Ghz,omitempty"`
	CPUArch        string  `json:"cpu_arch,omitempty"`
	Hypervisor     string  `json:"hypervisor,omitempty"`
	CPUModel       string  `json:"cpu_model,omitempty"`
	RAMSize        float64 `json:"ram_size_Gb,omitempty"`
	RAMFreq        float64 `json:"ram_freq,omitempty"`
	GPU            int     `json:"gpu,omitempty"`
	GPUModel       string  `json:"gpu_model,omitempty"`
	DiskSize       int64   `json:"disk_size_Gb,omitempty"`
	MainDiskType   string  `json:"main_disk_type"`
	MainDiskSpeed  float64 `json:"main_disk_speed_MBps"`
	SampleNetSpeed float64 `json:"sample_net_speed_KBps"`
	EphDiskSize    int64   `json:"eph_disk_size_Gb"`
	PricePerHour   float64 `json:"price_in_dollars_hour"`
}

func createCPUInfo(output string) (*CPUInfo, error) {
	str := strings.TrimSpace(output)

	tokens := strings.Split(str, "î")
	if len(tokens) < 9 {
		return nil, fmt.Errorf("parsing error: '%s'", str)
	}
	info := CPUInfo{}
	var err error
	info.NumberOfCPU, err = strconv.Atoi(tokens[0])
	if err != nil {
		return nil, fmt.Errorf("Parsing error: NumberOfCPU='%s' (from '%s')", tokens[0], str)
	}
	info.NumberOfCore, err = strconv.Atoi(tokens[1])
	if err != nil {
		return nil, fmt.Errorf("Parsing error: NumberOfCore='%s' (from '%s')", tokens[1], str)
	}
	info.NumberOfSocket, err = strconv.Atoi(tokens[2])
	if err != nil {
		return nil, fmt.Errorf("Parsing error: NumberOfSocket='%s' (from '%s')", tokens[2], str)
	}
	info.NumberOfCore = info.NumberOfCore * info.NumberOfSocket
	info.CPUFrequency, err = strconv.ParseFloat(tokens[3], 64)
	if err != nil {
		return nil, fmt.Errorf("Parsing error: CpuFrequency='%s' (from '%s')", tokens[3], str)
	}
	info.CPUFrequency = math.Floor(info.CPUFrequency*100) / 100000

	info.CPUArch = tokens[4]
	info.Hypervisor = tokens[5]
	info.CPUModel = tokens[6]
	info.RAMSize, err = strconv.ParseFloat(tokens[7], 64)
	if err != nil {
		return nil, fmt.Errorf("Parsing error: RAMSize='%s' (from '%s')", tokens[7], str)
	}

	memInGb := info.RAMSize / 1024 / 1024
	info.RAMSize = math.Floor(memInGb*100) / 100
	info.RAMFreq, err = strconv.ParseFloat(tokens[8], 64)
	if err != nil {
		info.RAMFreq = 0
	}
	gpuTokens := strings.Split(tokens[9], "%")
	nb := len(gpuTokens)
	if nb > 1 {
		info.GPUModel = strings.TrimSpace(gpuTokens[0])
		info.GPU = nb - 1
	}

	info.DiskSize, err = strconv.ParseInt(tokens[10], 10, 64)
	if err != nil {
		info.DiskSize = 0
	}
	info.DiskSize = info.DiskSize / 1024 / 1024 / 1024

	info.EphDiskSize, err = strconv.ParseInt(tokens[11], 10, 64)
	if err != nil {
		info.EphDiskSize = 0
	}
	info.EphDiskSize = info.EphDiskSize / 1024 / 1024 / 1024

	info.MainDiskSpeed, err = strconv.ParseFloat(tokens[12], 64)
	if err != nil {
		info.MainDiskSpeed = 0
	}

	rotational, err := strconv.ParseInt(tokens[13], 10, 64)
	if err != nil {
		info.MainDiskType = ""
	} else {
		if rotational == 1 {
			info.MainDiskType = "HDD"
		} else {
			info.MainDiskType = "SSD"
		}
	}

	nsp, err := strconv.ParseFloat(tokens[14], 64)
	if err != nil {
		info.SampleNetSpeed = 0
	} else {
		info.SampleNetSpeed = nsp / 1000 / 8
	}

	info.PricePerHour = 0

	return &info, nil
}

// RunScanner ...
func RunScanner() {
	var targetedProviders []string
	theProviders, _ := providers.Tenants()

	for tenantName := range theProviders {
		if strings.Contains(tenantName, "-scannable-gpu-test") {
			targetedProviders = append(targetedProviders, tenantName)
		}
	}

	// TODO Enable when several brokerd instances can run in parallel
	/*
		var wtg sync.WaitGroup

		wtg.Add(len(targetedProviders))

		for _, tenantName := range targetedProviders {
			fmt.Printf("Working with tenant %s\n", tenantName)
			go analyzeTenant(&wtg, tenantName)
		}

		wtg.Wait()
	*/

	for _, tenantName := range targetedProviders {
		fmt.Printf("Working with tenant %s\n", tenantName)
		analyzeTenant(nil, tenantName)
	}

	collect()
}

func analyzeTenant(group *sync.WaitGroup, theTenant string) error {
	if group != nil {
		defer group.Done()
	}

	service, err := providers.GetService(theTenant)
	if err != nil {
		log.Warnf("Unable to get service for tenant '%s': %s", theTenant, err.Error())
		return err
	}

	err = dumpImages(service, theTenant)
	if err != nil {
		return err
	}

	err = dumpTemplates(service, theTenant)
	if err != nil {
		return err
	}

	templates, err := service.ListTemplates(true)

	img, err := service.SearchImage("Ubuntu 16.04")
	if err != nil {
		log.Warnf("No image here...")
		return err
	}

	// Prepare network

	there := true
	var net *model.Network

	netName := "scanner"
	if net, err = service.GetNetwork(netName); net != nil && err == nil {
		there = true
		log.Warnf("Network '%s' already there", netName)
	} else {
		there = false
	}

	if !there {
		net, err = service.CreateNetwork(model.NetworkRequest{
			CIDR:      "192.168.0.0/24",
			IPVersion: IPVersion.IPv4,
			Name:      netName,
		})
		if err == nil {
			defer service.DeleteNetwork(net.ID)
		} else {
			return errors.Wrapf(err, "Error waiting for server ready: %v", err)
		}
		if net == nil {
			return errors.Errorf("Failure creating network")
		}
	}

	_ = os.MkdirAll(utils.AbsPathify("$HOME/.safescale/scanner"), 0777)

	var wg sync.WaitGroup

	concurrency := math.Min(4, float64(len(templates)/2))
	sem := make(chan bool, int(concurrency))

	hostAnalysis := func(template model.HostTemplate) error {
		defer wg.Done()
		if net != nil {

			// TODO Remove this later

			/*
				if template.Name != "s1-2" {
					return nil
				}
			*/

			log.Printf("Checking template %s\n", template.Name)

			hostName := "scanhost-" + template.Name
			host, err := service.CreateHost(model.HostRequest{
				ResourceName: hostName,
				PublicIP:     true,
				ImageID:      img.ID,
				TemplateID:   template.ID,
				Networks:     []*model.Network{net},
			})

			defer service.DeleteHost(hostName)
			if err != nil {
				log.Warnf("template [%s] host '%s': error creation: %v\n", template.Name, hostName, err.Error())
				return err
			}

			sshSvc := server.NewSSHService(service)
			ssh, err := sshSvc.GetConfig(host.ID)
			if err != nil {
				log.Warnf("template [%s] host '%s': error reading SSHConfig: %v\n", template.Name, hostName, err.Error())
				return err
			}
			nerr := ssh.WaitServerReady(time.Duration(concurrency-1) * time.Minute)
			if nerr != nil {
				log.Warnf("template [%s] : Error waiting for server ready: %v", template.Name, nerr)
				return nerr
			}
			c, err := ssh.Command(cmd)
			if err != nil {
				log.Warnf("template [%s] : Problem creating ssh command: %v", template.Name, err)
				return err
			}
			_, cout, _, err := c.Run()
			if err != nil {
				log.Warnf("template [%s] : Problem running ssh command: %v", template.Name, err)
				return err
			}

			daCPU, err := createCPUInfo(cout)
			if err != nil {
				log.Warnf("template [%s] : Problem building cpu info: %v", template.Name, err)
				return err
			}

			daCPU.TemplateName = template.Name
			daCPU.TemplateID = template.ID
			daCPU.ImageID = img.ID
			daCPU.ImageName = img.Name
			daCPU.TenantName = theTenant
			daCPU.LastUpdated = time.Now().Format(time.RFC850)

			daOut, err := json.MarshalIndent(daCPU, "", "\t")
			if err != nil {
				log.Warnf("template [%s] : Problem marshaling json data: %v", template.Name, err)
				return err
			}

			nerr = ioutil.WriteFile(utils.AbsPathify("$HOME/.safescale/scanner/"+theTenant+"#"+template.Name+".json"), daOut, 0666)
			if nerr != nil {
				log.Warnf("template [%s] : Error writing file: %v", template.Name, nerr)
				return nerr
			}
		} else {
			return errors.New("No gateway network !")
		}

		return nil
	}

	wg.Add(len(templates))

	for _, target := range templates {
		sem <- true
		go func(inner model.HostTemplate) {
			defer func() { <-sem }()
			err = hostAnalysis(inner)
			if err != nil {
				log.Warnf("Error running scanner: %+v", err)
			}
		}(target)
	}

	for i := 0; i < cap(sem); i++ {
		sem <- true
	}

	wg.Wait()

	return nil
}

func dumpTemplates(service *providers.Service, tenant string) error {
	_ = os.MkdirAll(utils.AbsPathify("$HOME/.safescale/scanner"), 0777)

	type TemplateList struct {
		Templates []model.HostTemplate `json:"templates,omitempty"`
	}

	templates, err := service.ListTemplates(false)
	if err != nil {
		return err
	}

	content, err := json.Marshal(TemplateList{
		Templates: templates,
	})

	f := fmt.Sprintf("$HOME/.safescale/scanner/%s-templates.json", tenant)
	f = utils.AbsPathify(f)

	err = ioutil.WriteFile(f, content, 0666)
	if err != nil {
		return err
	}

	return nil
}

func dumpImages(service *providers.Service, tenant string) error {
	_ = os.MkdirAll(utils.AbsPathify("$HOME/.safescale/scanner"), 0777)

	type ImageList struct {
		Images []model.Image `json:"images,omitempty"`
	}

	images, err := service.ListImages(false)
	if err != nil {
		return err
	}

	content, err := json.Marshal(ImageList{
		Images: images,
	})

	f := fmt.Sprintf("$HOME/.safescale/scanner/%s-images.json", tenant)
	f = utils.AbsPathify(f)

	err = ioutil.WriteFile(f, content, 0666)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	log.Printf("%s version %s\n", os.Args[0], VERSION)
	log.Printf("built %s\n", BUILD_DATE)

	RunScanner()
}
