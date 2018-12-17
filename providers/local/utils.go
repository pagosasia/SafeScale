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

package local

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
)

type VmInfo struct {
	publicIp string
}

type vmInfoWaiter_ struct {
	listner     *net.Listener
	chansByName map[string](chan VmInfo)
	mutex       sync.Mutex
}

var vmInfoWaiter = vmInfoWaiter_{
	chansByName: map[string](chan VmInfo){},
}

func (iw *vmInfoWaiter_) Register(name string) chan VmInfo {
	channel := make(chan VmInfo)

	iw.mutex.Lock()
	iw.chansByName[name] = channel
	iw.mutex.Unlock()

	return channel
}

func (iw *vmInfoWaiter_) Deregister(name string) error {
	iw.mutex.Lock()
	channel, found := iw.chansByName[name]
	if found {
		delete(iw.chansByName, name)
		close(channel)
	}
	iw.mutex.Unlock()

	if !found {
		return fmt.Errorf("Nothing registered with the name %s", name)
	}
	return nil
}

func GetInfoWaiter(port int) (*vmInfoWaiter_, error) {
	if vmInfoWaiter.listner == nil {
		listner, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err != nil {
			return nil, fmt.Errorf("Failed to open a tcp connection : %s", err.Error())
		}
		vmInfoWaiter.listner = &listner

		go infoHandler()
	}

	return &vmInfoWaiter, nil
}

func infoHandler() {
	for {
		conn, err := (*vmInfoWaiter.listner).Accept()
		if err != nil {
			panic(fmt.Sprintf("Info handler, Error accepting: %s", err.Error()))
		}

		go func(net.Conn) {
			defer func() {
				conn.Close()
			}()

			buffer := make([]byte, 1024)

			nbChars, err := conn.Read(buffer)
			if err != nil {
				panic(fmt.Sprintf("Info handler, Error reading: %s", err.Error()))
			}

			message := string(buffer[0:nbChars])
			message = strings.Trim(message, "\n")
			splittedMessage := strings.Split(message, "|")
			hostName := splittedMessage[0]
			ip := splittedMessage[1]
			info := VmInfo{
				publicIp: ip,
			}
			vmInfoWaiter.mutex.Lock()
			channel, found := vmInfoWaiter.chansByName[hostName]
			vmInfoWaiter.mutex.Unlock()
			if !found {
				panic(fmt.Sprintf("Info handler, Recived info from an unregisterd host: \n%s", message))
			}
			channel <- info
			err = vmInfoWaiter.Deregister(hostName)
			if err != nil {
				panic(fmt.Sprintf("Info handler, Error deregistering: %s", err.Error()))
			}
		}(conn)
	}
}