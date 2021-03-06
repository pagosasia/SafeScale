package main

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)


func Test_Flexible_Basic(t *testing.T) {
	runOnlyInIntegrationTest("TEST_FLEXIBLE")
	defer tearDown()

	brokerd_launched, err := isBrokerdLaunched()
	if !brokerd_launched {
		fmt.Println("This requires that you launch brokerd in background and set the tenant")
		require.True(t, brokerd_launched)
	}
	require.Nil(t, err)

	in_path, err := canBeRun("broker")
	require.Nil(t, err)

	require.True(t, brokerd_launched)
	require.True(t, in_path)

	out, err := getOutput("broker tenant list")
	require.Nil(t, err)
	require.True(t, len(out) > 0)

	out, err = getOutput("broker tenant get")
	if err != nil {
		fmt.Println("This requires that you set the right tenant before launching the tests")
		require.Nil(t, err)
	}
	require.True(t, len(out) > 0)

	out, err = getOutput("broker network list")
	require.Nil(t, err)

	fmt.Println("Creating network crazy...")

	out, err = getOutput("broker network create crazy")
	require.Nil(t, err)

	out, err = getOutput("broker network create crazy")
	require.NotNil(t, err)
	require.True(t, strings.Contains(out, "already exist"))

	fmt.Println("Creating VM easyVM...")

	out, err = getOutput("broker host create easyvm --public --net crazy --cpu 4 --ram 50 ")
	require.Nil(t, err)

	out, err = getOutput("broker host create easyvm --public --net crazy --cpu 4 --ram 50 ")
	require.NotNil(t, err)
	require.True(t, strings.Contains(out, "already exist") || strings.Contains(out, "already used"))

	out, err = getOutput("broker host inspect easyvm")
	require.Nil(t, err)

	easyvm := HostInfo{}
	json.Unmarshal([]byte(out), &easyvm)

	fmt.Println("Creating VM complexvm...")

	out, err = getOutput("broker host create complexvm --public --net crazy")
	require.Nil(t, err)

	out, err = getOutput("broker host create complexvm --public --net crazy")
	require.NotNil(t, err)
	require.True(t, strings.Contains(out, "already exist") || strings.Contains(out, "already used"))

	out, err = getOutput("broker nas list")
	require.Nil(t, err)

	fmt.Println("Creating NAS bnastest...")

	out, err = getOutput("broker nas  create bnastest easyvm")
	require.Nil(t, err)

	out, err = getOutput("broker nas  mount bnastest complexvm")
	require.Nil(t, err)

	out, err = getOutput("broker nas list")
	require.Nil(t, err)
	require.True(t, strings.Contains(out, "bnastest"))

	out, err = getOutput("broker nas inspect bnastest")
	require.Nil(t, err)

	require.True(t, strings.Contains(out, "bnastest"))
	require.True(t, strings.Contains(out, "easyvm"))
	require.True(t, strings.Contains(out, "complexvm"))

	out, err = getOutput("broker nas  umount bnastest complexvm")
	require.Nil(t, err)

	out, err = getOutput("broker nas inspect bnastest")
	require.Nil(t, err)

	require.True(t, strings.Contains(out, "bnastest"))
	require.True(t, strings.Contains(out, "easyvm"))
	require.False(t, strings.Contains(out, "complexvm"))

	out, err = getOutput("broker nas delete bnastest ")
	require.Nil(t, err)

	out, err = getOutput("broker nas list")
	require.Nil(t, err)
	require.True(t, strings.Contains(out, "null"))

	out, err = getOutput("broker volume list")
	require.Nil(t, err)
	require.True(t, strings.Contains(out, "null"))

	fmt.Println("Creating Volume volumetest...")

	out, err = getOutput("broker volume  create volumetest")
	require.Nil(t, err)

	out, err = getOutput("broker volume list")
	require.Nil(t, err)
	require.True(t, strings.Contains(out, "volumetest"))

	out, err = getOutput("broker volume  attach  volumetest easyvm ")
	require.Nil(t, err)

	out, err = getOutput("broker volume delete volumetest")
	require.NotNil(t, err)
	require.True(t, strings.Contains(out, "still attached"))

	out, err = getOutput("broker volume inspect volumetest")
	require.Nil(t, err)
	require.True(t, strings.Contains(out, easyvm.ID))

	out, err = getOutput("broker volume  detach  volumetest easyvm ")
	require.Nil(t, err)

	out, err = getOutput("broker volume inspect volumetest")
	require.Nil(t, err)
	require.False(t, strings.Contains(out, easyvm.ID))

	out, err = getOutput("broker volume delete volumetest")
	require.Nil(t, err)

	out, err = getOutput("broker volume list")
	require.Nil(t, err)
	require.True(t, strings.Contains(out, "null"))

	out, err = getOutput("broker ssh run easyvm -c \"uptime\"")
	require.Nil(t, err)
	require.True(t, strings.Contains(out, "0 users"))

	out, err = getOutput("broker host delete easyvm")
	require.Nil(t, err)
	require.True(t, strings.Contains(out, "deleted"))

	out, err = getOutput("broker host delete complexvm")
	require.Nil(t, err)
	require.True(t, strings.Contains(out, "deleted"))

	out, err = getOutput("broker host delete gw-crazy")
	require.Nil(t, err)
	require.True(t, strings.Contains(out, "deleted"))

	out, err = getOutput("broker network delete crazy")
	require.Nil(t, err)
	require.True(t, strings.Contains(out, "deleted"))
}
