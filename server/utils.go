/**
 * @copyright       Copyright (C) 360.cn 2016
 * @author         YunLong.Lee    <liyunlong-s@360.cn>
 * @version        0.5
 */
package main

import (
	"encoding/binary"
	"encoding/json"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/cfg"
)

const (
	METHOD_GET    HttpMethod = "GET"
	METHOD_POST   HttpMethod = "POST"
	METHOD_PUT    HttpMethod = "PUT"
	METHOD_DELETE HttpMethod = "DELETE"
)

type HttpMethod string

func jsonify(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func callApi(method HttpMethod, hostname string, apiPath string, params interface{}, retVal interface{}) error {
	if apiPath[0] != '/' {
		return errors.New("api path must starts with /")
	}
	url := "http://" + hostname + apiPath
	client := &http.Client{Transport: http.DefaultTransport}

	b, err := json.Marshal(params)
	if err != nil {
		return errors.Trace(err)
	}

//  fmt.Println( string(b) )

	req, err := http.NewRequest(string(method), url, strings.NewReader(string(b)))
	if err != nil {
		return errors.Trace(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("can't connect to app server %s, please check 'app_server_ip' is correct in config file", app_server_ip)
		return errors.Trace(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Trace(err)
	}

	if resp.StatusCode == 200 {
		err := json.Unmarshal(body, retVal)
		if err != nil {
			return errors.Trace(err)
		}
		return nil
	}

	return errors.Errorf("http status code %d, %s", resp.StatusCode, string(body))
}

func getCurrentPath() string {
	file, _ := exec.LookPath(os.Args[0])
	path, _ := filepath.Abs(file)
	index := strings.LastIndex(path, string(os.PathSeparator))
	ret := path[:index]
	return ret
}

func createPidFile(filename string) error {
	if len(filename) == 0 {
		return nil
	}

	err := os.MkdirAll(path.Dir(filename), 0755)
	if err != nil {
		return errors.Trace(err)
	}

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Trace(err)
	}
	defer f.Close()

	if _, err = f.WriteString(fmt.Sprintf("%d", os.Getpid())); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func removePidFile(filename string) error {
	if len(filename) == 0 {
		return nil
	}

	_, err := os.Stat(filename)
	if err != nil {
        glog.Info("write: %v", err)
		return err
	}
	err = os.Remove(filename)
	if err != nil {
        glog.Info("write: %v", err)
		return err
	}
	return nil
}

func printRequest(r *http.Request) {
	glog.Infof("========================================================")
	glog.Infof("receiving handshake request from %s", r.RemoteAddr)
	glog.Infof("========================================================")
	glog.Infof("%s %s %s", r.Method, r.URL, r.Proto)
	for k, v := range r.Header {
        glog.Info("%s: ", k)
		for _, vv := range v {
            glog.Info("%s", vv)
		}
	}
}

func Int64ToBytes(i int64) []byte {
	var buf = make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(i))
	return buf
}

func BytesToInt64(buf []byte) int64 {
	return int64(binary.LittleEndian.Uint64(buf))
}

func GenerateId() int64 {
	return time.Now().UnixNano()
}

func GenUid() string {
    b := make([]byte, 16)
    rand.Read(b)
    return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func jsonRet(output map[string]interface{}) (int, string) {
	b, err := json.Marshal(output)
	if err != nil {
		glog.Warning(err)
	}
	return 200, string(b)
}

func jsonRetFail(errCode int, msg string) (int, string) {
	return jsonRet(map[string]interface{}{
		"ret": errCode,
		"msg": msg,
	})
}

func jsonRetSucc() (int, string) {
	return jsonRet(map[string]interface{}{
		"ret": 0,
		"msg": "OK",
	})
}

func InitConfigFromFile(filename string) (*cfg.Cfg, error) {
	ret := cfg.NewCfg(filename)
	if err := ret.Load(); err != nil {
		return nil, errors.Trace(err)
	}
	return ret, nil
}

type Strings []string
func (s1 Strings) Eq(s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true
}
