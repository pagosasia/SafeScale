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

package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	safeutils "github.com/CS-SI/SafeScale/utils"
	"github.com/nanobox-io/golang-scribble"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"

	"github.com/CS-SI/SafeScale/providers/api"
	"github.com/CS-SI/SafeScale/providers/model"
	"github.com/CS-SI/SafeScale/providers/model/enums/HostState"
	"github.com/CS-SI/SafeScale/providers/model/enums/VolumeState"
	"github.com/CS-SI/SafeScale/providers/objectstorage"
)

// Service Client High level service
type Service struct {
	api.ClientAPI
	ObjectStorage  objectstorage.Location
	MetadataBucket objectstorage.Bucket
}

// // FromClient contructs a Service instance from a ClientAPI
// func FromClient(clt api.ClientAPI) *Service {
// 	return &Service{
// 		ClientAPI: clt,
// 	}
// }

const (
	//CoreDRFWeight is the Dominant Resource Fairness weight of a core
	CoreDRFWeight float32 = 1.0
	//RAMDRFWeight is the Dominant Resource Fairness weight of 1 GB of RAM
	RAMDRFWeight float32 = 1.0 / 8.0
	//DiskDRFWeight is the Dominant Resource Fairness weight of 1 GB of Disk
	DiskDRFWeight float32 = 1.0 / 16.0
)

// RankDRF computes the Dominant Resource Fairness Rank of an host template
func RankDRF(t *model.HostTemplate) float32 {
	fc := float32(t.Cores)
	fr := t.RAMSize
	fd := float32(t.DiskSize)
	return fc*CoreDRFWeight + fr*RAMDRFWeight + fd*DiskDRFWeight
}

// ByRankDRF implements sort.Interface for []HostTemplate based on
// the Dominant Resource Fairness
type ByRankDRF []model.HostTemplate

func (a ByRankDRF) Len() int           { return len(a) }
func (a ByRankDRF) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByRankDRF) Less(i, j int) bool { return RankDRF(&a[i]) < RankDRF(&a[j]) }

// HostAccess an host and the SSH Key Pair
type HostAccess struct {
	Host    *model.Host
	Key     *model.KeyPair
	User    string
	Gateway *HostAccess
}

// GetAccessIP returns the access IP
func (access *HostAccess) GetAccessIP() string {
	return access.Host.GetAccessIP()
}

// // ServerRequest used to create a server
// type ServerRequest struct {
// 	Name string `json:"name,omitempty"`
// 	// NetworksIDs list of the network IDs the host must be connected
// 	Networks []model.Network `json:"networks,omitempty"`
// 	// PublicIP a flg telling if the host must have a public IP is
// 	PublicIP bool `json:"public_ip,omitempty"`
// 	// TemplateID the UUID of the template used to size the host (see SelectTemplates)
// 	Template mode.HostTemplate `json:"sizing,omitempty"`
// 	// ImageID  is the UUID of the image that contains the server's OS and initial state.
// 	OSName string `json:"os_name,omitempty"`
// 	// Gateway through which the server can be connected
// 	Gateway *HostAccess `json:"gateway,omitempty"`
// }

// WaitHostState waits an host achieve state
func (svc *Service) WaitHostState(hostID string, state HostState.Enum, timeout time.Duration) error {
	var err error

	timer := time.After(timeout)
	next := true
	host := model.NewHost()
	host.ID = hostID
	for next {
		host, err = svc.GetHost(host)
		if err != nil {
			return err
		}
		if host.LastState == state {
			return nil
		}
		if host.LastState == HostState.ERROR {
			return fmt.Errorf("host in error state")
		}
		select {
		case <-timer:
			return fmt.Errorf("timeout waiting host '%s' to reach state '%s'", host.Name, state.String())
		default:
			time.Sleep(1)
		}
	}
	return err
}

// WaitHostState waits an host achieve state
// func (svc *Service) WaitHostState(hostID string, state HostState.Enum, timeout time.Duration) (*api.host, error) {
// 	cout := make(chan int)
// 	stop := make(chan bool)
// 	hostc := make(chan *api.Host)
// 	fmt.Println(timeout)
// 	var host *api.Host
// 	var err error
// 	go pollHost(srv, hostID, state, cout, stop, hostc)
// 	stop <- false
// 	timer := time.After(timeout)
// 	finish := false
// 	for !finish {
// 		select {
// 		case res := <-cout:
// 			if res == 0 {
// 				stop <- true
// 				err = fmt.Errorf("host in error state")
// 				finish = true
// 			}
// 			if res == 1 {
// 				fmt.Println("State achieved")
// 				stop <- true
// 				host = <-hostc
// 				fmt.Println("host received")
// 				finish = true
// 			}
// 			if res == 2 {
// 				stop <- false
// 			}
// 		case <-timer:
// 			stop <- true
// 			err = fmt.Errorf("Timeout")
// 			finish = true
// 		default:
// 		}
// 	}
// 	fmt.Println("receive result")
// 	<-cout
// 	fmt.Println("End of wait")
// 	return host, err
// }

// func sendResul(cout chan int, res int) {
// 	cout <- res
// 	fmt.Println("result sent ", res)
// }

// func pollHost(client api.ClientAPI, hostID string, state HostState.Enum, cout chan int, stop chan bool, hostc chan *api.Host) {
// 	finish := false
// 	fmt.Println("Start polling")
// 	for !finish {
// 		res := -1
// 		if finish {
// 			return
// 		}
// 		finish = <-stop

// 		fmt.Println("Get host")
// 		host, err := client.GetHost(hostID)
// 		if err != nil {
// 			log.Print(err)
// 			res = 0
// 		} else if host.State == state {
// 			res = 1
// 		} else if host.State == HostState.ERROR {
// 			res = 0
// 		} else {
// 			res = 2
// 		}
// 		fmt.Println(host.State)
// 		sendResul(cout, res)

// 		if res == 1 {
// 			fmt.Println("send host")
// 			hostc <- host
// 		}
// 		fmt.Println("end")
// 	}
// }

// WaitVolumeState waits an host achieve state
func (svc *Service) WaitVolumeState(volumeID string, state VolumeState.Enum, timeout time.Duration) (*model.Volume, error) {
	cout := make(chan int)
	next := make(chan bool)
	vc := make(chan *model.Volume)

	go pollVolume(svc, volumeID, state, cout, next, vc)
	for {
		select {
		case res := <-cout:
			if res == 0 {
				//next <- false
				return nil, fmt.Errorf("Error getting host state")
			}
			if res == 1 {
				//next <- false
				return <-vc, nil
			}
			if res == 2 {
				next <- true
			}
		case <-time.After(timeout):
			next <- false
			return nil, &model.ErrTimeout{Message: "Wait host state timeout"}
		}
	}
}

func pollVolume(client api.ClientAPI, volumeID string, state VolumeState.Enum, cout chan int, next chan bool, hostc chan *model.Volume) {
	for {

		v, err := client.GetVolume(volumeID)
		if err != nil {

			cout <- 0
			return
		}
		if v.State == state {
			cout <- 1
			hostc <- v
			return
		}
		cout <- 2
		if !<-next {
			return
		}
	}
}

// SelectTemplatesBySize select templates satisfying sizing requirements
// returned list is ordered by size fitting
func (svc *Service) SelectTemplatesBySize(sizing model.SizingRequirements, force bool) ([]model.HostTemplate, error) {
	templates, err := svc.ListTemplates(false)
	var selectedTpls []model.HostTemplate
	scannerTemplates := map[string]bool{}
	if err != nil {
		return nil, err
	}

	askedForSpecificScannerInfo := sizing.MinGPU > 0 || sizing.MinFreq != 0
	if askedForSpecificScannerInfo {
		_ = os.MkdirAll(safeutils.AbsPathify("$HOME/.safescale/scanner"), 0777)
		db, err := scribble.New(safeutils.AbsPathify("$HOME/.safescale/scanner/db"), nil)
		if err != nil {
			if !force {
				fmt.Println("Problem accessing Scanner database: ignoring GPU and Freq parameters...")
				log.Warnf("Problem creating / accessing Scanner database, ignoring for now...: %v", err)
			} else {
				noHostError := fmt.Sprintf("Unable to create a host with '%d' GPUs and '%f' GHz clock frequency !, problem accessing Scanner database: %v", sizing.MinGPU, sizing.MinFreq, err)
				log.Error(noHostError)
				return nil, errors.New(noHostError)
			}
		} else {
			image_list, err := db.ReadAll("images")
			if err != nil {
				if !force {
					fmt.Println("Problem accessing Scanner database: ignoring GPU and Freq parameters...")
					log.Warnf("Error reading Scanner database: %v", err)
				} else {
					noHostError := fmt.Sprintf("Unable to create a host with '%d' GPUs and '%f' GHz clock frequency !, problem listing images from Scanner database: %v", sizing.MinGPU, sizing.MinFreq, err)
					log.Error(noHostError)
					return nil, errors.New(noHostError)
				}
			} else {
				images := []model.StoredCPUInfo{}
				for _, f := range image_list {
					imageFound := model.StoredCPUInfo{}
					if err := json.Unmarshal([]byte(f), &imageFound); err != nil {
						fmt.Println("Error", err)
					}

					if imageFound.GPU < int(sizing.MinGPU) {
						continue
					}

					if imageFound.CPUFrequency < float64(sizing.MinFreq) {
						continue
					}

					images = append(images, imageFound)
				}

				if !force && (len(images) == 0) {
					noHostError := fmt.Sprintf("Unable to create a host with '%d' GPUs and '%f' GHz clock frequency !, no such host found with those specs !!", sizing.MinGPU, sizing.MinFreq)
					log.Error(noHostError)
					return nil, errors.New(noHostError)
				}

				for _, image := range images {
					scannerTemplates[image.TemplateID] = true
				}
			}
		}
	}

	log.Debugf("Looking for machine with: %d core%s, %.01f GB RAM, and %d GB Disk",
		sizing.MinCores, safeutils.Plural(sizing.MinCores), sizing.MinRAMSize, sizing.MinDiskSize)

	for _, template := range templates {
		if template.Cores >= sizing.MinCores && (template.DiskSize == 0 || template.DiskSize >= sizing.MinDiskSize) && template.RAMSize >= sizing.MinRAMSize {
			if _, ok := scannerTemplates[template.ID]; ok || !askedForSpecificScannerInfo {
				selectedTpls = append(selectedTpls, template)
			}
		} else {
			log.Debugf("Discard machine template '%s' with : %d cores, %f RAM, and %d Disk", template.Name, template.Cores, template.RAMSize, template.DiskSize)
		}
	}

	sort.Sort(ByRankDRF(selectedTpls))
	return selectedTpls, nil
}

// FilterImages search an images corresponding to OS Name
func (svc *Service) FilterImages(filter string) ([]model.Image, error) {
	imgs, err := svc.ListImages(false)
	if err != nil {
		return nil, err
	}
	if len(filter) == 0 {
		return imgs, nil
	}
	fimgs := []model.Image{}
	//fields := strings.Split(strings.ToUpper(osname), " ")
	for _, img := range imgs {
		//score := 1 / float64(smetrics.WagnerFischer(strings.ToUpper(img.Name), strings.ToUpper(osname), 1, 1, 2))
		//score := smetrics.JaroWinkler(strings.ToUpper(img.Name), strings.ToUpper(osname), 0.7, 5)
		//score := matchScore(fields, strings.ToUpper(img.Name))
		score := SimilarityScore(filter, img.Name)
		if score > 0.5 {
			fimgs = append(fimgs, img)
		}

	}
	return fimgs, nil

}

// SearchImage search an image corresponding to OS Name
func (svc *Service) SearchImage(osname string) (*model.Image, error) {
	imgs, err := svc.ListImages(false)
	if err != nil {
		return nil, err
	}
	maxscore := 0.0
	maxi := -1
	//fields := strings.Split(strings.ToUpper(osname), " ")
	for i, img := range imgs {
		//score := 1 / float64(smetrics.WagnerFischer(strings.ToUpper(img.Name), strings.ToUpper(osname), 1, 1, 2))
		//score := smetrics.JaroWinkler(strings.ToUpper(img.Name), strings.ToUpper(osname), 0.7, 5)
		//score := matchScore(fields, strings.ToUpper(img.Name))
		score := SimilarityScore(osname, img.Name)
		if score > maxscore {
			maxscore = score
			maxi = i
		}

	}
	//fmt.Println(fields, len(fields))
	//fmt.Println(len(fields))
	if maxscore < 0.5 || maxi < 0 || len(imgs) == 0 {
		return nil, fmt.Errorf("unable to find an image matching %s", osname)
	}

	log.Printf("Selected image: '%s' (ID='%s')", imgs[maxi].Name, imgs[maxi].ID)
	return &imgs[maxi], nil
}

// CreateHostWithKeyPair creates an host
func (svc *Service) CreateHostWithKeyPair(request model.HostRequest) (*model.Host, *model.KeyPair, error) {
	_, err := svc.GetHostByName(request.ResourceName)
	if err == nil {
		return nil, nil, model.ResourceAlreadyExistsError("Host", request.ResourceName)
	}

	//Create temporary key pair
	kpNameuuid, err := uuid.NewV4()
	if err != nil {
		return nil, nil, err
	}

	kpName := kpNameuuid.String()
	kp, err := svc.CreateKeyPair(kpName)
	if err != nil {
		return nil, nil, err
	}
	//defer svc.DeleteKeyPair(kpName)

	// Create host
	hostReq := model.HostRequest{
		ResourceName:   request.ResourceName,
		HostName:       request.HostName,
		ImageID:        request.ImageID,
		KeyPair:        kp,
		PublicIP:       request.PublicIP,
		Networks:       request.Networks,
		DefaultGateway: request.DefaultGateway,
		TemplateID:     request.TemplateID,
	}
	host, err := svc.CreateHost(hostReq)
	if err != nil {
		return nil, nil, err
	}
	return host, kp, nil
}

// ListHostsByName list hosts by name
func (svc *Service) ListHostsByName() (map[string]*model.Host, error) {
	hosts, err := svc.ListHosts()
	if err != nil {
		return nil, err
	}
	hostMap := make(map[string]*model.Host)
	for _, host := range hosts {
		hostMap[host.Name] = host
	}
	return hostMap, nil
}

// CreateBucket creates an object container
func (svc *Service) CreateBucket(bucketName string) error {
	if svc.ObjectStorage == nil {
		panic("svc.Location is nil!")
	}
	_, err := svc.ObjectStorage.CreateBucket(bucketName)
	if err != nil {
		return err
	}
	return nil
}

// DeleteBucket deletes an object container
func (svc *Service) DeleteBucket(bucketName string) error {
	if svc.ObjectStorage == nil {
		panic("svc.Location is nil!")
	}
	return svc.ObjectStorage.DeleteBucket(bucketName)
}

// ListBuckets list object containers
func (svc *Service) ListBuckets() ([]string, error) {
	if svc.ObjectStorage == nil {
		panic("svc.ObjectStorage is nil!")
	}
	return svc.ObjectStorage.ListBuckets(objectstorage.NoPrefix)
}

// GetBucket returns info about the Bucket
func (svc *Service) GetBucket(bucketName string) (objectstorage.Bucket, error) {
	if svc.ObjectStorage == nil {
		panic("svc.Location is nil!")
	}
	return svc.ObjectStorage.GetBucket(bucketName)
}

// PutObject put an object into a Bucket
func (svc *Service) PutObject(bucketName string, obj model.Object) error {
	if svc.ObjectStorage == nil {
		panic("svc.Location is nil!")
	}
	b, err := svc.ObjectStorage.GetBucket(bucketName)
	if err != nil {
		return err
	}
	_, err = b.WriteObject(obj.Name, obj.Content, obj.Size, nil)
	return err
}

// UpdateObjectMetadata update an object into  object container
func (svc *Service) UpdateObjectMetadata(bucketName string, obj model.Object) error {
	// Stow doesn't allow Object Metadata only update for now
	return fmt.Errorf("Not implemented")
}

// GetObject get object content from a Bucket
func (svc *Service) GetObject(bucketName string, objectName string, ranges []model.Range) (*model.Object, error) {
	if svc.ObjectStorage == nil {
		panic("svc.Location is nil!")
	}
	o, err := svc.ObjectStorage.GetObject(bucketName, objectName)
	if err != nil {
		return nil, err
	}
	mo := model.Object{
		ID:       o.GetID(),
		Name:     o.GetName(),
		Metadata: o.GetMetadata(),
		Size:     o.GetSize(),
		ETag:     o.GetETag(),
	}
	mo.LastModified, err = o.GetLastUpdate()
	if err != nil {
		return nil, err
	}
	if ranges == nil || len(ranges) == 0 {
		r := model.NewRange(0, 0)
		ranges = []model.Range{r}
	}
	buf := bytes.NewBuffer(nil)
	for _, r := range ranges {
		err = o.Read(buf, int64(*r.From), int64(*r.To))
		if err != nil {
			return nil, err
		}
	}
	if mo.Size != int64(buf.Len()) {
		return nil, fmt.Errorf("object size doesn't match with size of read data")
	}
	mo.Content = bytes.NewReader(buf.Bytes())
	return &mo, nil
}

// GetObjectMetadata get object metadata from a Bucket
func (svc *Service) GetObjectMetadata(bucketName string, objectName string) (*model.Object, error) {
	if svc.ObjectStorage == nil {
		panic("svc.ObjectStorage is nil!")
	}
	o, err := svc.ObjectStorage.GetObject(bucketName, objectName)
	if err != nil {
		return nil, err
	}
	mo := model.Object{
		ID:       o.GetID(),
		Name:     o.GetName(),
		Metadata: o.GetMetadata(),
		Size:     o.GetSize(),
		ETag:     o.GetETag(),
	}
	mo.LastModified, err = o.GetLastUpdate()
	if err != nil {
		return nil, err
	}
	return &mo, nil
}

// ListObjects list objects of a container
func (svc *Service) ListObjects(bucketName string, filter model.ObjectFilter) ([]string, error) {
	if svc.ObjectStorage == nil {
		panic("svc.Location is nil!")
	}
	return svc.ObjectStorage.ListObjects(bucketName, filter.Path, filter.Prefix)
}

// CopyObject copies an object
func (svc *Service) CopyObject(bucketNameSrc, objectSrc, objectDst string) error {
	// stow doesn't allow object copy for now
	return fmt.Errorf("not implemented")
}

// DeleteObject delete an object from a container
func (svc *Service) DeleteObject(bucketName, objectName string) error {
	if svc.ObjectStorage == nil {
		panic("svc.Location is nil!")
	}
	return svc.ObjectStorage.DeleteObject(bucketName, objectName)
}

func runeIndexes(s string, r rune) []int {
	positions := []int{}
	for i, l := range s {
		if l == r {
			positions = append(positions, i)
		}
	}
	return positions

}

func runesIndexes(ref string, s string) [][]int {
	positions := [][]int{}
	uref := strings.ToUpper(ref)
	us := strings.ToUpper(s)
	for _, r := range uref {
		if r != ' ' {
			positions = append(positions, runeIndexes(us, r))
		}

	}
	return positions
}

func recPossiblePathes(positions [][]int, level int) [][]int {
	newPathes := [][]int{}
	if level >= len(positions) {
		return [][]int{
			[]int{},
		}
	}
	pathes := recPossiblePathes(positions, level+1)
	if len(positions[level]) == 0 {
		for _, path := range pathes {
			newPathes = append(newPathes, append([]int{-1}, path...))
		}
	} else {
		for _, idx := range positions[level] {
			for _, path := range pathes {
				newPathes = append(newPathes, append([]int{idx}, path...))
			}
		}
	}

	return newPathes
}

func possiblePathes(positions [][]int) [][]int {
	return recPossiblePathes(positions, 0)
}

func bestPath(pathes [][]int, size int) (int, int) {
	if len(pathes) == 0 {
		return -1, 10000
	}
	minD := distance(pathes[0], size)
	bestI := 0
	for i, p := range pathes {
		d := distance(p, size)
		if d < minD {
			minD = d
			bestI = i
		}
	}
	return bestI, minD
}

func distance(path []int, size int) int {
	d := 0
	previous := path[0]
	for _, index := range path {
		if index < 0 {
			d += size
		} else {
			di := index - previous
			d += di
			if di < 0 {
				d += di + size
			}
		}
		previous = index
	}
	return d
}

func score(d int, rsize int) float64 {
	return float64(rsize-1) / float64(d)
}

// SimilarityScore computes a similariy score between 2 strings
func SimilarityScore(ref string, s string) float64 {
	size := len(s)
	rsize := len(ref)
	if rsize > size {
		return SimilarityScore(s, ref)
	}
	_, d := bestPath(possiblePathes(runesIndexes(ref, s)), size)
	ds := math.Abs(float64(size-rsize)) / float64(rsize)
	return score(d, len(ref)) / (math.Log10(10 * (1. + ds)))
}

// InitializeBucket creates the Object Storage Container/Bucket that will store the metadata
// id contains a unique identifier of the tenant (something coming from the provider, not the tenant name)
func InitializeBucket(svc api.ClientAPI, location objectstorage.Location) error {
	cfg, err := svc.GetCfgOpts()
	if err != nil {
		fmt.Printf("failed to get client options: %s\n", err.Error())
	}
	anon, found := cfg.Get("MetadataBucket")
	if !found || anon.(string) == "" {
		return fmt.Errorf("failed to get value of option 'MetadataBucket'")
	}
	_, err = location.CreateBucket(anon.(string))
	if err != nil {
		return err
	}
	return nil
}
