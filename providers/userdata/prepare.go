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

package userdata

//go:generate rice embed-go

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/CS-SI/SafeScale/providers/api"
	"github.com/CS-SI/SafeScale/providers/model"
	"github.com/CS-SI/SafeScale/utils"
	rice "github.com/GeertJohan/go.rice"
)

// userData is the structure to apply to userdata.sh template
type userData struct {
	// User is the name of the default user (api.DefaultUser)
	User string
	// Key is the public key used to create the Host
	PublicKey string
	//PKey is the private key used to create the Host
	PrivateKey string
	// ConfIF, if set to true, configure all interfaces to DHCP
	ConfIF bool
	// IsGateway, if set to true, activate IP frowarding
	IsGateway bool
	// AddGateway, if set to true, configure default gateway
	AddGateway bool
	// DNSServers contains the list of DNS servers to use
	// Used only if IsGateway is true
	DNSServers []string
	//CIDR contains the cidr of the network
	CIDR string
	// GatewayIP is the IP of the gateway
	GatewayIP string
	// Password for the user gpac (for troubleshoot use, useable only in console)
	Password string
	// HostName contains the name wanted as host name (default == name of the Cloud resource)
	HostName string
}

var userdataTemplate *template.Template

// Prepare prepares the initial configuration script executed by cloud compute resource
func Prepare(
	client api.ClientAPI, request model.HostRequest, kp *model.KeyPair, cidr string,
) ([]byte, error) {

	// Generate password for user gpac
	var (
		gpacPassword              string
		err                       error
		anon                      interface{}
		ok                        bool
		autoHostNetworkInterfaces bool
		useLayer3Networking       = true
		dnsList                   []string
	)
	//if debug
	if true {
		gpacPassword = "SafeScale"
	} else {
		gpacPassword, err = utils.GeneratePassword(16)
		if err != nil {
			return nil, fmt.Errorf("failed to generate password: %s", err.Error())
		}
	}

	// Determine Gateway IP
	ip := ""
	if request.DefaultGateway != nil {
		ip = request.DefaultGateway.GetPrivateIP()
	}

	config, err := client.GetCfgOpts()
	if err != nil {
		return nil, nil
	}
	anon, ok = config.Get("AutoHostNetworkInterfaces")
	if ok {
		autoHostNetworkInterfaces = anon.(bool)
	}
	anon, ok = config.Get("UseLayer3Networking")
	if ok {
		useLayer3Networking = anon.(bool)
	}
	anon, ok = config.Get("DNSList")
	if ok {
		dnsList = anon.([]string)
	} else {
		dnsList = []string{"1.1.1.1"}
	}

	if userdataTemplate == nil {
		b, err := rice.FindBox("../userdata/scripts")
		if err != nil {
			return nil, err
		}
		tmplString, err := b.String("userdata.sh")
		if err != nil {
			return nil, fmt.Errorf("error loading script template: %s", err.Error())
		}
		userdataTemplate, err = template.New("userdata").Parse(tmplString)
		if err != nil {
			return nil, fmt.Errorf("error parsing script template: %s", err.Error())
		}
	}

	data := userData{
		User:       model.DefaultUser,
		PublicKey:  strings.Trim(kp.PublicKey, "\n"),
		PrivateKey: strings.Trim(kp.PrivateKey, "\n"),
		ConfIF:     !autoHostNetworkInterfaces,
		IsGateway:  request.DefaultGateway == nil && request.Networks[0].Name != model.SingleHostNetworkName && !useLayer3Networking,
		AddGateway: !request.PublicIP && !useLayer3Networking,
		DNSServers: dnsList,
		CIDR:       cidr,
		GatewayIP:  ip,
		Password:   gpacPassword,

		//HostName:   request.Name,
	}

	dataBuffer := bytes.NewBufferString("")
	err = userdataTemplate.Execute(dataBuffer, data)
	if err != nil {
		return nil, err
	}
	return dataBuffer.Bytes(), nil
}

func initUserdataTemplate() error {
	if userdataTemplate != nil {
		// Already loaded
		return nil
	}

	var (
		err         error
		box         *rice.Box
		userdataStr string
	)

	box, err = rice.FindBox("../userdata/scripts")
	if err == nil {
		userdataStr, err = box.String("userdata.sh")
		if err == nil {
			userdataTemplate, err = template.New("user_data").Parse(userdataStr)
			if err == nil {
				return nil
			}
		}
	}
	return err
}
