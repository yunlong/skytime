/**
 * @author         YunLong.Lee    <yunlong.lee@163.com>
 * @version        0.5
 */
package main

import (
	"fmt"
	"runtime"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/env"
	"github.com/go-zookeeper/zk"
	"github.com/zkhelper"
	"github.com/cfg"
)

////////////////// skytime connect to etcd or zookeeper ///////////////////////
var (
	globalEnv  env.Env
	globalConn zkhelper.Conn
	livingNode string
)

func GetCoordLock(coordConn zkhelper.Conn, productName string) zkhelper.ZLocker {
    coordPath := fmt.Sprintf("/zk/skytime/db_%s/LOCK", productName)
    ret := zkhelper.CreateMutex(coordConn, coordPath)
    return ret
}

func CreateCoordConn(config *cfg.Cfg ) zkhelper.Conn {
    globalEnv = env.LoadRebornEnv(config)
	conn, err := globalEnv.NewCoordConn()
	if err != nil {
		glog.Errorf("Failed to create coordinator connection: " + err.Error())
	}
	return conn
}

func createDashboardNode() {

	// make sure root dir is exists
	rootDir := fmt.Sprintf("/zk/skytime/db_%s", globalEnv.ProductName())
	zkhelper.CreateRecursive(globalConn, rootDir, "", 0, zkhelper.DefaultDirACLs())

	coordPath := fmt.Sprintf("%s/skytime_status", rootDir)

	// make sure we're the only one dashboard

	/***
	timeoutCh := time.After(60 * time.Second)

	for {
		if exists, _, ch, _ := conn.ExistsW(coordPath); exists {
			data, _, _ := conn.Get(coordPath)

			if checkDashboardAlive(data) {
				return errors.Errorf("dashboard already exists: %s", string(data))
			} else {
				log.Warningf("dashboard %s exists in zk, wait it removed", data)

				select {
				case <-ch:
				case <-timeoutCh:
					return errors.Errorf("wait existed dashboard %s removed timeout", string(data))
				}
			}
		} else {
			break
		}
	}

	***/

	content := fmt.Sprintf(`{"cpu": "%d", "cgo": %v, "goroutine": %d, "version": "%s", "pid":%d}`,
		runtime.NumCPU(),
		runtime.NumCgoCall(),
		runtime.NumGoroutine(),
		runtime.Version(),
		syscall.Getpid())
	pathCreated, err := globalConn.Create(coordPath, []byte(content), zk.FlagEphemeral, zkhelper.DefaultFileACLs())
	glog.Infof("skytime heartbeat node %s created, data %s, err %v", pathCreated, string(content), err)

}

func delPeerNode(peerAddr string) {

	rootDir := fmt.Sprintf("/zk/skytime/db_%s", globalEnv.ProductName())
	zkhelper.CreateRecursive(globalConn, rootDir, "", 0, zkhelper.DefaultDirACLs())

	peer_id := peerAddr
	coordPath := fmt.Sprintf("%s/skytime_peers_%s", rootDir, peer_id)

	_, stat, err := globalConn.Exists(coordPath)
	if err != nil {
		glog.Errorf("conn.Exists: %v", err)
	}

	if stat != nil {
		glog.Errorf("%s should be deleted, got: %v", coordPath, stat)
	}

	if err := globalConn.Delete(coordPath, -1); err != nil {
		glog.Errorf("conn.Delete: %v", err)
	}

	glog.Infof("skytime peer node %s deleted, err %v", coordPath, err)

}

func addPeerNode(peerAddr string) {

	rootDir := fmt.Sprintf("/zk/skytime/db_%s", globalEnv.ProductName())
	zkhelper.CreateRecursive(globalConn, rootDir, "", 0, zkhelper.DefaultDirACLs())

	peer_id := peerAddr
	coordPath := fmt.Sprintf("%s/skytime_peers_%s", rootDir, peer_id)

	content := fmt.Sprintf(`{"peer_ip": "%s", "status": "%s", "register_time": "%s"}`,
		peerAddr,
		"online",
		time.Now().Format("2006-01-02 15:04:05"))

	pathCreated, err := globalConn.Create(coordPath, []byte(content), zk.FlagEphemeral, zkhelper.DefaultFileACLs())
	glog.Infof("skytime peer node %s created, data %s, err %v", pathCreated, string(content), err)

}
