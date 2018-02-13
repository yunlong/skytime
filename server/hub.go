/**
 * @copyright       Copyright (C) 360.cn 2016
 * @author         YunLong.Lee    <liyunlong-s@360.cn>
 * @version        0.5
 */
package main

import (
	"fmt"
	"time"
	"sync"
    "bytes"
    "strings"
	"encoding/binary"

	"github.com/golang/glog"

	"github.com/consistent"
	"github.com/gorilla/websocket"
	"github.com/juju/errors"
	"github.com/cfg"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 8192
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

///////////////////////////////////////////////////////////////////////////////
var (
	max_retry_ttl = 8
	max_wait_ttl  = 64
)

type write_msg_t struct {
	msg_body 	[]byte
	retry_times	int
	wait_ttl  	int
}

func send_push_msg_to_peer( c *connection, ack_seq uint32, payload []byte, is_br_all bool ) error {

	write_buf := make([]byte, 9 + len(payload))
  	binary.BigEndian.PutUint32(write_buf, uint32(ack_seq) )
	b4 :=  byte(0x1 << 4 | 0x1 << 1 | 0x1)

    /**
    if strings.Compare(string(payload), "refresh_peer_id") == 0 {
        b4 = byte(0x1 << 4 | 0x1 << 3 | 0x1)
    }
    **/

    if is_br_all {
	    b4 = byte(0x1 << 4 | 0x0 << 3 | 0x1)
    }

	write_buf[4] = b4
	body_len := len(payload)
	binary.BigEndian.PutUint32(write_buf[5:], uint32(body_len))
    copy(write_buf[9:], payload)

	if err := c.write(websocket.TextMessage, write_buf); err != nil {
		glog.Infof("write:%v", err)
		return err
	}

	return nil
}

func send_ack_msg_to_peer( c *connection, ack_seq uint32 ) error {

	write_buf := make([]byte, 5)
	binary.BigEndian.PutUint32(write_buf, uint32(ack_seq) )
	b4 :=  byte(0x1 << 4 | 0x3 << 1 | 0x1)
	write_buf[4] = b4
	if err := c.write(websocket.TextMessage, write_buf); err != nil {
		glog.Infof("write:%v", err)
		return err
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
/// connection is an middleman between the websocket connection and the hub. ///
////////////////////////////////////////////////////////////////////////////////
type connection struct {
	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
//	send chan []byte
	send chan *push_msg_t

    // mid of skylar endpoint
    peer_id string

	recv_ack_seq chan int
    write_msg_Q map[int]*write_msg_t

    notify_mu chan bool
}

// readPump pumps messages from the websocket connection to the hub.
func (c *connection) readPump() {

	defer func() {
		hrooms.unregister(c)
		c.ws.Close()
	}()

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(3 * pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(3 * pongWait)); return nil })
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				glog.Infof("error: %v", err)
			}
			break
		}

		hrooms.broadcast_Q <- message
	}
}

func (c *connection) checkPongMessage() {

	defer func() {
		hrooms.unregister(c)
		c.ws.Close()
	}()

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(3 * pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(3 * pongWait))
        if enable_verbose_log > 0 {
            glog.Infof("receiving PongMessage from peer %s", c.ws.RemoteAddr())
        }
		return nil
	})

	for {
		mt, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				glog.Infof("error: %v", err)
			}
			break
		}

		if mt == websocket.TextMessage {

            ack_seq  := binary.BigEndian.Uint32(message[0:4])
            msg_ver  := byte( message[4] >> 4 )
            msg_type := byte( message[4]  & 0xf  >> 1 )
            svr_flag := byte( message[4]  & 0x1 )

            if msg_type == 3 {

                // recving ack message from client
                if enable_verbose_log > 0 {
                    glog.Infof("receiving ACK_SEQ %d msg_ver %d msg_type %d svr_flag %d from peer %s",
                        ack_seq, msg_ver, msg_type, svr_flag, c.ws.RemoteAddr())
                }

                c.recv_ack_seq <- int( ack_seq )

                // ack counter
                hrooms.recv_ack_ch <- int(ack_seq)
            }

            if msg_type == 2 {

                body_len := binary.BigEndian.Uint32(message[5:9])

                if enable_verbose_log > 0 {
                    glog.Infof("receiving notify message \"%s\" with ack_seq %d msg_ver %d msg_type %d svr_flag %d body_len %d from peer %s",
                        message[9:], ack_seq, msg_ver, msg_type, svr_flag, body_len, c.ws.RemoteAddr() )

                    glog.Infof("send ack message \"%s\" with ack_seq %d msg_ver %d msg_type %d svr_flag %d body_len %d from peer %s",
                        message[9:], ack_seq, msg_ver, msg_type, svr_flag, body_len, c.ws.RemoteAddr() )
                }

            //  time.Sleep(300 * time.Millisecond)
                notify_ack_msg := &notify_ack_msg_t{ c: c, ack_seq: int(ack_seq) }
			    hrooms.notify_ack_Q <- notify_ack_msg

                if enable_verbose_log > 0 {
                    glog.Infof("ready send notify message[9:] body to provider by http POST method")
                }

            }
        }
	}
}

// write writes a message with the given message type and payload.
func (c *connection) write(mt int, payload []byte) error {

    //  !!! don't touch these code
    if enable_ack_rto == 1 {
        <-c.notify_mu
	    c.ws.SetWriteDeadline(time.Now().Add(writeWait))
        ret := c.ws.WriteMessage(mt, payload)
        c.notify_mu<-true
        return ret
    } else {
	    c.ws.SetWriteDeadline(time.Now().Add(writeWait))
    	return c.ws.WriteMessage(mt, payload)
    }
}

// Retransmission TimeOut Pump pumps messages from the hub to the websocket connection.
func (c *connection) writeRTO() {

	ping_ticker := time.NewTicker( time.Duration( max_ping_interval ) * time.Second )
	rto_ticker  := time.NewTicker(1 * time.Second)

	defer func() {
		ping_ticker.Stop()
		rto_ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {

		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
                if enable_verbose_log > 0 {
				    glog.Infof("sending CloseMessage to peer %s", c.ws.RemoteAddr())
                }
				return
			}

			write_seq := message.msg_id

            if message.is_br_all {

                if enable_verbose_log > 0 {
                    glog.Infof("sending broadcast_all Message with ack_seq %d to peer %s", write_seq, c.ws.RemoteAddr())
                }

                send_push_msg_to_peer( c, uint32(write_seq), message.msg_body, message.is_br_all )

            } else {

                if enable_verbose_log > 0 {
                    glog.Infof("sending broadcast_peer Message with ack_seq %d to peer %s", write_seq, c.ws.RemoteAddr())
                }

			    next_write_msg := &write_msg_t{msg_body: message.msg_body, retry_times: max_retry_ttl, wait_ttl: max_wait_ttl}
			    c.write_msg_Q[ write_seq ] = next_write_msg
			    send_push_msg_to_peer( c, uint32(write_seq), message.msg_body, message.is_br_all )

			    hrooms.send_msg_ch <- int(write_seq)
            }

		case <-ping_ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				glog.Infof("error: %v", err)
				return
			}

            if enable_verbose_log > 0 {
			    glog.Infof("sending PingMessage to peer %s", c.ws.RemoteAddr())
            }

		case ack_seq := <-c.recv_ack_seq:
			if _, ok := c.write_msg_Q[ack_seq]; ok {
				delete(c.write_msg_Q, ack_seq)

                if enable_verbose_log > 0 {
				    glog.Infof("RECV ACK_SEQ OK from %s : DROP ACK_SEQ %d Message from write_msg_Q", c.ws.RemoteAddr(), ack_seq)
                }
			}

		case <-rto_ticker.C:
			for ack_seq, msg := range c.write_msg_Q {
				msg.wait_ttl--
				if msg.wait_ttl <= 0 {
			        if _, ok := c.write_msg_Q[ack_seq]; ok {
					    delete(c.write_msg_Q, ack_seq)

                        if enable_verbose_log > 0 {
					        glog.Infof("RECV ACK_SEQ TIMEOUT from %s: DROP ACK_SEQ %d Message from write_msg_Q", c.ws.RemoteAddr(), ack_seq)
                        }
                    }
				}

				if msg.wait_ttl < (msg.retry_times - 1) * ( max_wait_ttl / max_retry_ttl ) {

					// retry send push_msg_t to peer
                    if enable_verbose_log > 0 {
					    glog.Infof("retry sending Message with ACK_SEQ %d to peer %s", ack_seq, c.ws.RemoteAddr())
                    }

					send_push_msg_to_peer( c, uint32(ack_seq), msg.msg_body, false )
			 		hrooms.send_msg_ch <- ack_seq

					msg.retry_times--

				}
			}
		}
	}
}

func (c *connection) writePump() {

	ping_ticker := time.NewTicker( time.Duration( max_ping_interval ) * time.Second )

	defer func() {
		ping_ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})

                if enable_verbose_log > 0 {
				    glog.Infof("sending CloseMessage to peer %s", c.ws.RemoteAddr())
                }

				return
			}

            if enable_verbose_log > 0 {
		        glog.Infof("sending Message \"%s\" to peer %s", message.msg_body, c.ws.RemoteAddr())
            }

			if err := c.write(websocket.TextMessage, message.msg_body); err != nil {
				glog.Infof("write:%v", err)
				return
			}

		case <-ping_ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				glog.Infof("error: %v", err)
				return
			}
            if enable_verbose_log > 0 {
		        glog.Infof("sending PingMessage to peer %s", c.ws.RemoteAddr())
            }
		}
	}
}

/////////////////////////////////////////////////////////////
//// app_provider maintains all app detail about business ///
/////////////////////////////////////////////////////////////
type app_provider_t struct {
    app_id      string
    app_name    string
    app_desc    string
  	app_callback string
}

func NewProvider() *app_provider_t {
	return &app_provider_t{
        app_id  :       "1234134123",
		app_name:       "edr_policy",
		app_desc:       "edr_policy",
		app_callback: 	"http://x.x.x.x/uri",
	}
}

func register_app_provider( ap *app_provider_t ) {
}

func unregister_app_provider( ap *app_provider_t ) {
}

///////////////////////////////////////////////////////////////////////////////////////////////
//// hub maintains the set of active connections and broadcasts messages to the connections ///
///////////////////////////////////////////////////////////////////////////////////////////////

// reserve the invalid peer_id info
/**
type invalid_peer_t struct {
	org_peer_id string
    new_peer_id string
}
*/

type hub struct {

	// Registered connections.
    connections map[string]*connection

	// Inbound messages from the connections of http/telnet push.
//	broadcast chan []byte
	broadcast chan *push_msg_t

	// Register requests from the connections of websocket client.
	register chan *connection

	// Unregister requests from connections of websocket client.
	unregister chan *connection

	peercnt int64

    // connections for the repeated peer_id, it's next plan
//   sconns map[*connection]string

}

func Newhub() *hub {
	return &hub{
	//	broadcast:   make(chan []byte, 1024),
		broadcast:   make(chan *push_msg_t, 4096),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
        connections: make(map[string]*connection),
		peercnt:     0,
    //  sconns:     make(map[*connection]string),
	}
}

func (h *hub) run() {

	rate := time.Second / 1000
	burstLimit := 10000
	tick := time.NewTicker(rate)
	defer tick.Stop()
	throttle := make(chan time.Time, burstLimit)

	go func() {
		for t := range tick.C {
			select {
			case throttle <- t:
			default:
			}
		}  // exits after tick.Stop()
	}()

	for {
		select {
		case c := <-h.register:
            h.connections[ c.peer_id ] = c
            h.peercnt++

            /**
            is_online := hrooms.peer_is_online( c.peer_id )
            if is_online == true {
			    if _, ok := h.connections[c.peer_id]; ok {
				    delete(h.connections, c.peer_id)
				    close(c.send)
				    h.peercnt--
			    }

                // push connection to sconns of hub
                h.sconns[ c ] = c.peer_id
                if enable_verbose_log > 0 {
                    glog.Infof("push connection of the repeated peer %s:%s to sconn of hub", c.peer_id, c.ws.RemoteAddr())
                }
                go func() {
                    peer_ids := make([]string,0)
                    peer_ids = append(peer_ids, c.peer_id)
                    time.Sleep(100 * time.Millisecond)
                    hrooms.broadcast_peers( peer_ids, []byte("refresh_peer_id"), c )
                }()

            } else {
                h.connections[ c.peer_id ] = c
            }

            h.peercnt++
            **/

		case c := <-h.unregister:
			if _, ok := h.connections[c.peer_id]; ok {

				delete(h.connections, c.peer_id)
				close(c.send)
				h.peercnt--

			}

            /**
            else {

                // pop connection from sconns of hub
                if _, ok := h.sconns[ c ]; ok {
                    delete(h.sconns, c)
                    close(c.send)
                }

                if enable_verbose_log > 0 {
                    glog.Infof("pop connection of the repeated peer %s:%s to sconn of hub", c.peer_id, c.ws.RemoteAddr())
                }

				h.peercnt--

            }
            **/

		case m := <-h.broadcast:
			for _, c := range h.connections {

				<-throttle  // rate limit our Service.Method RPCs
				select {
				case c.send <- m:
				default:
					close(c.send)
				    delete(h.connections, c.peer_id)
				    h.peercnt--
				}
			}

            /**
            for c := range h.sconns {

                <-throttle  // rate limit our Service.Method RPCs
                select {
                case c.send <- m:
                default:
                    close(c.send)
                    delete(h.sconns, c)
                    h.peercnt--
                }
            }
            **/
		}
	}
}

///////////////////////////////////////////////////////////////////////////////////////////
const (
    MAX_ROOM_NUM = 100
    RESERVED_ROOM_NUM = 10
	MAX_PEER_IN_ROOM  = 10240
)

type push_msg_t struct {
    msg_id int
    msg_body []byte
    is_br_all   bool
}

type notify_ack_msg_t struct {
    c   *connection
    ack_seq int
}

type msg_stat_t struct {
    send_msg_cnt int
    recv_ack_cnt int
}

type hub_room_stat_t struct {
	total_msg_cnt      int64
	total_msg_size     int64
	total_consume_time int64
	total_peer_cnt     int64
}

type hub_rooms struct {
	cstHash   *consistent.Consistent
	rooms     map[string]*hub
	rooms_ids []string
	stats     hub_room_stat_t

	broadcast_mu chan bool
	broadcast_Q  chan []byte

	stat_rwlock sync.RWMutex
    msg_stats map[int]*msg_stat_t
    send_msg_ch chan int
    recv_ack_ch chan int

    notify_ack_Q  chan *notify_ack_msg_t
}

var (
	hrooms = &hub_rooms{}
)

func (hr *hub_rooms) run( config *cfg.Cfg ) {

    ping_interval, err := config.ReadInt("max_ping_interval", -1)
    if err != nil {
        glog.Infof("%v", err)
    }

    max_ping_interval = ping_interval
    if max_ping_interval < 0 {
        max_ping_interval = int( pingPeriod / time.Second )
    }
	glog.Infof("max_ping_interval is %d second \n", max_ping_interval)

    ///////////////////////////////////////////////////////////////////
    room_limit, err := config.ReadInt("max_room_limit", 10)
    if err != nil {
        glog.Infof("%v", err)
    }
    max_room_limit = room_limit
	glog.Infof("max_room_limit is %d \n", max_room_limit)

	if max_room_limit > MAX_ROOM_NUM {
		max_room_limit = MAX_ROOM_NUM
	}

	if max_room_limit < RESERVED_ROOM_NUM {
		max_room_limit = RESERVED_ROOM_NUM
	}
	hr.cstHash = consistent.New()
	glog.Infof("starting create %d hub rooms .....\n", max_room_limit)

	hr.rooms_ids = make([]string, 0)
	for i := 0; i < max_room_limit; i++ {
		room_id := fmt.Sprintf("room_%d", i)
		hr.rooms_ids = append(hr.rooms_ids, room_id)
	}

	hr.rooms = make(map[string]*hub)
	for _, v := range hr.rooms_ids {
		hr.cstHash.Add(v)
		hr.rooms[v] = Newhub()
		go hr.rooms[v].run()
	}
    ///////////////////////////////////////////////////////////////////

	hr.broadcast_Q = make(chan []byte, 40960)
	hr.broadcast_mu = make(chan bool, 1)
	hr.broadcast_mu <- true

    hr.msg_stats = make(map[int]*msg_stat_t)
    hr.send_msg_ch = make(chan int)
    hr.recv_ack_ch = make(chan int)

    ///////////////////////////////////////////////////////////////////
    wait_ttl, err := config.ReadInt("max_wait_ttl", 64)
    if err != nil {
        glog.Infof("%v", err)
    }
    max_wait_ttl = wait_ttl
    glog.Infof("max_wait_ttl is %d", max_wait_ttl)

    retry_ttl, err := config.ReadInt("max_retry_ttl", 8)
    if err != nil {
        glog.Infof("%v", err)
    }
    max_retry_ttl = retry_ttl
    glog.Infof("max_retry_ttl is %d", max_retry_ttl)

    ///////////////////////////////////////////////////////////////////
	hr.notify_ack_Q = make(chan *notify_ack_msg_t, 8192)

}

func (hr *hub_rooms) msg_ack_stat() {

	for {
		select {

		case send_msg_seq := <-hr.send_msg_ch:
			hr.stat_rwlock.Lock()
			if v, ok := hr.msg_stats[ send_msg_seq ]; ok {
				v.send_msg_cnt++
			}
			hr.stat_rwlock.Unlock()


		case recv_ack_seq := <-hr.recv_ack_ch:
			hr.stat_rwlock.Lock()
			if v, ok := hr.msg_stats[ recv_ack_seq ]; ok {
				v.recv_ack_cnt++
			}
			hr.stat_rwlock.Unlock()
		}
	}

}

type peers_info_t struct {
     Peers []string `json:"mids"`
}

func (hr *hub_rooms) register(c *connection) error {

    /**
    is_online := hr.peer_is_online( c.peer_id )
    if is_online == true {

        c.ws.Close()

        if oc := hr.get_connection_by_peer_id( c.peer_id ); oc != nil {
            hr.unregister( oc )
            oc.ws.Close()
        }
        return nil
    }
    **/

	room_id := hr.get_room_id( c.peer_id )

	hr.rooms[room_id].register <- c
	hr.stats.total_peer_cnt++

    if enable_verbose_log > 0 {
	    glog.Infof("%s register new connection from peer %s\n", room_id, c.ws.RemoteAddr())
    }

    peers_info := &peers_info_t{
        Peers: []string{c.peer_id},
    }
    var v interface{}
    err := callApi(METHOD_POST, app_server_ip, "/api/client_online.json", peers_info, &v)
    if err != nil {
        return errors.Trace(err)
    }

    if enable_verbose_log > 0 {
        fmt.Println(jsonify(v))
    }

    return nil
}

func (hr *hub_rooms) unregister(c *connection) error {

	room_id := hr.get_room_id( c.peer_id )

	hr.rooms[room_id].unregister <- c
	hr.stats.total_peer_cnt--
    if enable_verbose_log > 0 {
	    glog.Infof("%s unregister a connection from peer %s\n", room_id, c.ws.RemoteAddr())
    }

    peers_info := &peers_info_t{
        Peers: []string{c.peer_id},
    }
    var v interface{}
    err := callApi(METHOD_POST, app_server_ip, "/api/client_offline.json", peers_info, &v)
    if err != nil {
        return errors.Trace(err)
    }

    if enable_verbose_log > 0 {
        fmt.Println(jsonify(v))
    }

    return nil
}

func (hr *hub_rooms) processNotifyMessage() {

	for {
		select {
		case notify_ack_msg, ok := <-hr.notify_ack_Q:
			if !ok {
				glog.Infof("broadcast Queue is Closed")
				return
			}

        //  !!! don't touch these code
        //  <-notify_ack_msg.c.notify_mu
		//	<-hr.broadcast_mu
            send_ack_msg_to_peer( notify_ack_msg.c, uint32(notify_ack_msg.ack_seq ))
		//	hr.broadcast_mu <- true
        //  notify_ack_msg.c.notify_mu <- true
		}
	}
}

func (hr *hub_rooms) broadcast() {

	for {
		select {
		case bmsg, ok := <-hr.broadcast_Q:
			if !ok {
				glog.Infof("broadcast Queue is Closed")
				return
			}

            if enable_verbose_log > 0 {
		        glog.Infof("sending broadcast Message \"%s\" at %s", bmsg, time.Now().Format("2006-01-02 15:04:05"))
            }

			<-hr.broadcast_mu

			hr.stats.total_msg_cnt++
            push_msg := &push_msg_t{ msg_id: int(hr.stats.total_msg_cnt), msg_body: make([]byte, len(bmsg)), is_br_all: true}
			copy(push_msg.msg_body, bmsg)

			hr.stat_rwlock.Lock()
            hr.msg_stats[ push_msg.msg_id ] = &msg_stat_t{send_msg_cnt:0, recv_ack_cnt:0}
			hr.stat_rwlock.Unlock()

			start := time.Now()
			for _, h := range hr.rooms {
				h.broadcast <- push_msg
			}

			diff := time.Now().Sub(start) / time.Millisecond
			hr.stats.total_msg_size += int64(len(bmsg))
			hr.stats.total_consume_time += int64(diff)

			hr.broadcast_mu <- true

            if enable_verbose_log > 0 {
                glog.Infof("push message take %d milliseconds", diff)
            }
		}
	}
}

func (hr *hub_rooms) broadcast_rooms( room_ids []string, br_msg []byte ) {

    if enable_verbose_log > 0 {
        glog.Infof("sending broadcast Message \"%s\" at %s", br_msg, time.Now().Format("2006-01-02 15:04:05"))
    }

	<-hr.broadcast_mu

	hr.stats.total_msg_cnt++
    push_msg := &push_msg_t{ msg_id: int(hr.stats.total_msg_cnt), msg_body: make([]byte, len(br_msg)), is_br_all: true}
	copy(push_msg.msg_body, br_msg)

	hr.stat_rwlock.Lock()
    hr.msg_stats[ push_msg.msg_id ] = &msg_stat_t{send_msg_cnt:0, recv_ack_cnt:0}
	hr.stat_rwlock.Unlock()

	start := time.Now()

	for _, room_id := range room_ids {
		if v, ok := hr.rooms[ room_id ]; ok {
			v.broadcast <- push_msg
		}
	}

	diff := time.Now().Sub(start) / time.Millisecond
	hr.stats.total_msg_size += int64(len(br_msg))
	hr.stats.total_consume_time += int64(diff)

	hr.broadcast_mu <- true

    if enable_verbose_log > 0 {
        glog.Infof("push message take %d milliseconds", diff)
    }

}

func (hr *hub_rooms) broadcast_peers( peer_ids []string, br_msg []byte, sc *connection) {

    if enable_verbose_log > 0 {
        glog.Infof("sending broadcast Message \"%s\" at %s", br_msg, time.Now().Format("2006-01-02 15:04:05"))
    }

	<-hr.broadcast_mu

	hr.stats.total_msg_cnt++
    push_msg := &push_msg_t{ msg_id: int(hr.stats.total_msg_cnt), msg_body: make([]byte, len(br_msg)), is_br_all: false}
	copy(push_msg.msg_body, br_msg)

	hr.stat_rwlock.Lock()
    hr.msg_stats[ push_msg.msg_id ] = &msg_stat_t{send_msg_cnt:0, recv_ack_cnt:0}
	hr.stat_rwlock.Unlock()

	start := time.Now()

	for _, peer_id := range peer_ids {
		room_id := hr.get_room_id( peer_id )

        if sc != nil {
            sc.send <- push_msg
        } else {
		    c := hr.rooms[ room_id ].connections[ peer_id ]
		    if c != nil {
			    glog.Infof("hits %s: %s", c.peer_id, c.ws.RemoteAddr().String() )
			    c.send <- push_msg
		    }
        }
	}

	diff := time.Now().Sub(start) / time.Millisecond
	hr.stats.total_msg_size += int64(len(br_msg))
	hr.stats.total_consume_time += int64(diff)

	hr.broadcast_mu <- true

    if enable_verbose_log > 0 {
        glog.Infof("push message take %d milliseconds", diff)
    }

}

func (hr *hub_rooms) stat() {

	for _, v := range hr.rooms_ids {
		glog.Infof("%s be connected to %d peers", v, hr.rooms[v].peercnt)
	}

	glog.Infof("hub_room be connected to %d peers", hr.stats.total_peer_cnt)

	if hr.stats.total_msg_cnt > 0 {
		glog.Infof("broadcast msg stats: \n total_msg_cnt is %d  \n total_msg_size is %d bytes, \n avg_consume_time is %d millisecond\n",
		    hr.stats.total_msg_cnt,
            hr.stats.total_msg_size,
            hr.stats.total_consume_time / hr.stats.total_msg_cnt)
	}

	hr.stat_rwlock.RLock()
	for i := 0; i <= int(hr.stats.total_msg_cnt); i++ {
		if v, ok := hr.msg_stats[ i ]; ok {
			glog.Infof("msg with ack_seq %d : send_msg_cnt is %d recv_ack_cnt is %d", i, v.send_msg_cnt, v.recv_ack_cnt)
		}
	}
	hr.stat_rwlock.RUnlock()

}

func (hr *hub_rooms) get_room_id( peer_id string ) (room_id string ) {
	room_id, err := hr.cstHash.Get( peer_id )
	if err != nil {
		glog.Errorf("hash error is %v", err)
	}
	return room_id
}

func (hr *hub_rooms) get_connection_by_peer_id( peer_id string ) ( *connection ) {
	room_id := hr.get_room_id( peer_id )
	c := hr.rooms[ room_id ].connections[ peer_id ]
	if c != nil {
        return c
	}
    return nil
}

func (hr *hub_rooms) peer_is_online( peer_id string) ( bool ) {

	room_id := hr.get_room_id( peer_id )
	c := hr.rooms[ room_id ].connections[ peer_id ]
	if c != nil {
        if enable_verbose_log > 0 {
	        glog.Infof("hits %s: %s", c.peer_id, c.ws.RemoteAddr().String() )
        }
        return true
	}

    return false
}

func (hr *hub_rooms) peers_is_online( peer_ids []string) ( string ) {

    var buf bytes.Buffer
	for _, peer_id := range peer_ids {
		room_id := hr.get_room_id( peer_id )
		c := hr.rooms[ room_id ].connections[ peer_id ]
		if c != nil {
            if enable_verbose_log > 0 {
			    glog.Infof("hits %s: %s", c.peer_id, c.ws.RemoteAddr().String() )
            }

            buf.WriteString(peer_id + ",")
		}
	}

    if buf.Len() > 32 {
        return strings.TrimSuffix(buf.String(), ",")
    }

    return ""
}

func (hr *hub_rooms) cron_detect_all_peers() error {

    for _, v := range hr.rooms_ids {
		if room, ok := hr.rooms[ v ]; ok {
		    for _, c := range room.connections {
                peers_info := &peers_info_t{
                    Peers: []string{c.peer_id},
                }
                var v interface{}
                err := callApi(METHOD_POST, app_server_ip, "/api/client_online.json", peers_info, &v)
                if err != nil {
                    return errors.Trace(err)
                }
                fmt.Println(jsonify(v))
		    }
        }
        time.Sleep(1000 * time.Millisecond)
    }

    return nil

}
