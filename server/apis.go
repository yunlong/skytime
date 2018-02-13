/**
 * @copyright       Copyright (C) 360.cn 2016
 * @author         YunLong.Lee    <liyunlong-s@360.cn>
 * @version        0.5
 */
package main

import (
	"encoding/json"
	"net/http"
	"runtime"
	"syscall"
    "strconv"
    "strings"
	"io/ioutil"
	"time"
)

func apiOverview(w http.ResponseWriter, r *http.Request) {

	info := make(map[string]interface{})

	info["product"] 			= "SkyTime"
	info["hub_room_cnt"] 		= len(hrooms.rooms_ids)
	info["online_peer_cnt"] 	= hrooms.stats.total_peer_cnt
	info["total_br_msg_cnt"] 	= hrooms.stats.total_msg_cnt
	info["total_br_msg_size"] 	= hrooms.stats.total_msg_size

	if hrooms.stats.total_msg_cnt > 0 {
		info["br_msg_avg_time"] = hrooms.stats.total_consume_time / hrooms.stats.total_msg_cnt
	} else {
		info["br_msg_avg_time"] = 0
	}

	if hrooms.stats.total_msg_cnt > 0 && hrooms.stats.total_consume_time > 1000 {
		info["ops"]				= hrooms.stats.total_msg_cnt / (hrooms.stats.total_consume_time / 1000)
	} else {
		info["ops"]				= 0
	}

	info["cpu_cores"]			= runtime.NumCPU()
	info["go_routines"]			= runtime.NumGoroutine()
	info["go_version"]			= runtime.Version()
	info["process_id"]			= syscall.Getpid()

	b, _ := json.MarshalIndent(info, " ", "  ")
//  w.Header().Add("X-Auth-Key", "12345") // normal header
//  w.Header().Add("X-Auth-Secret", "secret") // normal header
	w.Write(b)
}

type PeerInfo struct {
	PeerId string 	    `json:"peer_id"`
	PeerAddr string 	`json:"peer_addr"`
	PeerStatus string	`json:"peer_status"`
}

type PageData struct {
	Build   string
	Peers   []*PeerInfo
}

func apiRoomview(w http.ResponseWriter, r *http.Request) {

    is_ok := check_console_ipf( r.RemoteAddr )
    if !is_ok {
        return
    }

	var ret []*PeerInfo
	pos := strings.LastIndex(r.URL.Path, "/") + 1
	room_id := r.URL.Path[pos:]
	h := hrooms.rooms[room_id]
	for _, c := range h.connections {
		var peer = &PeerInfo{
			PeerId:  c.peer_id,
			PeerAddr:  c.ws.RemoteAddr().String(),
			PeerStatus: "online",
		}
		ret = append(ret, peer)
	}

	b, _ := json.MarshalIndent(ret, " ", "  ")
	w.Write(b)
}

type Room struct {
	RoomId  string `json:"room_id"`
	PeerNum int    `json:"peer_num"`
}

type RoomGroup struct {
	Rooms []*Room `json:"rooms"`
}

func (r *Room) String() string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}

func (rg *RoomGroup) String() string {
	b, _ := json.MarshalIndent(rg, "", "  ")
	return string(b) + "\n"
}

func apiGetServerGroup(w http.ResponseWriter, r *http.Request) {

    is_ok := check_console_ipf( r.RemoteAddr )
    if !is_ok {
        return
    }
	var ret []*Room

	for _, v := range hrooms.rooms_ids {
		var room = &Room{
			RoomId:  v,
			PeerNum: len(hrooms.rooms[v].connections),
		}

		ret = append(ret, room)
	}

	b, _ := json.MarshalIndent(ret, " ", "  ")
	w.Write(b)
}

func apiBroadcastAll(w http.ResponseWriter, r *http.Request) {

    is_ok := check_console_ipf( r.RemoteAddr )
    if !is_ok {
        return
    }

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
	    _, b := jsonRetFail(400, "Read Body failed")
	    w.Write([]byte(b))
        return
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		hrooms.broadcast_Q <- body
	}()

	_, b := jsonRetSucc()
	w.Write([]byte(b))

}

type range_br_msg struct {
	From 	int	 	`json:"from"`
	To		int	 	`json:"to"`
	Msg		string	`json:"msg"`
}

func apiBroadcastRange(w http.ResponseWriter, r *http.Request) {

    is_ok := check_console_ipf( r.RemoteAddr )
    if !is_ok {
        return
    }

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
	    _, b := jsonRetFail(400, "Read Body failed")
	    w.Write([]byte(b))
        return
	}

	room_ids := make([]string, 0)
	var dat range_br_msg
	if err := json.Unmarshal(body, &dat); err == nil {
		st := dat.From
		et := dat.To

		for i := st; i <= et; i++  {
			room_ids = append(room_ids, "room_" + strconv.Itoa( i ) )
		}
	}
	bmsg := dat.Msg

	go func() {
		time.Sleep(100 * time.Millisecond)
		hrooms.broadcast_rooms( room_ids, []byte(bmsg) )
	}()

	_, b := jsonRetSucc()
	w.Write([]byte(b))

}

type set_br_msg struct {
	From 	string 	`json:"from"`
	Msg		string	`json:"msg"`
}

func apiBroadcastSet(w http.ResponseWriter, r *http.Request) {

    is_ok := check_console_ipf( r.RemoteAddr )
    if !is_ok {
        return
    }

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
	    _, b := jsonRetFail(400, "Read Body failed")
	    w.Write([]byte(b))
        return
	}

	room_ids := make([]string, 0)
	var dat set_br_msg
	if err := json.Unmarshal(body, &dat); err == nil {
		rset := strings.Split(dat.From, ",")
		for _, v := range rset  {
			room_ids = append(room_ids, strings.TrimSpace("room_" + v ) )
		}
	}

	bmsg := dat.Msg

	go func() {
		time.Sleep(100 * time.Millisecond)
		hrooms.broadcast_rooms( room_ids, []byte(bmsg) )
	}()

	_, b := jsonRetSucc()
	w.Write([]byte(b))

}

type peers_br_msg struct {
	Peers 	string 	`json:"peers"`
	Msg		string	`json:"br_msg"`
}

func apiBroadcastPeers(w http.ResponseWriter, r *http.Request) {

    is_ok := check_console_ipf( r.RemoteAddr )
    if !is_ok {
        return
    }

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
	    _, b := jsonRetFail(400, "Read Body failed")
	    w.Write([]byte(b))
        return
	}

	peer_ids := make([]string, 0)
	var dat peers_br_msg
	if err := json.Unmarshal(body, &dat); err == nil {
		rset := strings.Split(dat.Peers, ",")
		for _, v := range rset  {
			peer_id := strings.TrimSpace( v )
		//	if len(peer_id) == 32 {
				peer_ids = append(peer_ids, peer_id)
		//	}
		}
	}

	bmsg := dat.Msg

	go func() {
		time.Sleep(100 * time.Millisecond)
		hrooms.broadcast_peers( peer_ids, []byte(bmsg), nil )
	}()

	_, b := jsonRetSucc()
	w.Write([]byte(b))

}


type online_peers_t struct {
	Peers 	string 	`json:"peers"`
}

func apiPeersIsOnline(w http.ResponseWriter, r *http.Request) {

    is_ok := check_console_ipf( r.RemoteAddr )
    if !is_ok {
        return
    }

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
	    _, b := jsonRetFail(400, "Read Body failed")
	    w.Write([]byte(b))
        return
	}

    peer_ids := make([]string, 0)
	var dat online_peers_t
	if err := json.Unmarshal(body, &dat); err == nil {
		rset := strings.Split(dat.Peers, ",")
		for _, v := range rset  {
			peer_id := strings.TrimSpace( v )
		//	if len(peer_id) == 32 {
				peer_ids = append(peer_ids, peer_id)
		//	}
		}
	}

    online_peer_ids := hrooms.peers_is_online( peer_ids )

	peers_info := make( map[string]interface{} )
    peers_info["is_online"] = online_peer_ids
    b, _ := json.MarshalIndent(peers_info, " ", "  ")
	w.Write(b)

}

func apiSyncBroadcastPeers(w http.ResponseWriter, r *http.Request) {

}

type HeartbeatMessage struct {
    Status string `json:"status"`
    Build  string `json:"build"`
    Uptime string `json:"uptime"`
}

func apiHeartbeat(rw http.ResponseWriter, r *http.Request) {

    is_ok := check_console_ipf( r.RemoteAddr )
    if !is_ok {
        return
    }

    uptime := time.Since(StartTime).String()
    err := json.NewEncoder(rw).Encode(HeartbeatMessage{"running", "Hello SkyTime", uptime})
    if err != nil {
        // glog.Errorf("Failed to write heartbeat message. Reason: %s", err.Error())
        json.NewEncoder(rw).Encode(HeartbeatMessage{"stopping", "OOM SkyTime", uptime})
    }

 }
