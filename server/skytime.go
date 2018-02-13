/**
 * @author         YunLong.Lee    <yunlong.lee@163.com>
 * @version        0.5
 */
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
    "os/exec"
    "path/filepath"
	"runtime"
	"syscall"
	"strconv"
    "strings"
	"time"

    "gopkg.in/tylerb/graceful.v1"
	"github.com/codegangsta/negroni"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
    "github.com/cfg"
)

var ws_banner string = `
0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-------+-+-------------+-------------------------------+
|F|R|R|R| opcode|M| Payload len |    Extended payload length    |
|I|S|S|S|  (4)  |A|     (7)     |             (16/64)           |
|N|V|V|V|       |S|             |   (if payload len==126/127)   |
| |1|2|3|       |K|             |                               |
+-+-+-+-+-------+-+-------------+ - - - - - - - - - - - - - - - +
|     Extended payload length continued, if payload len == 127  |
+ - - - - - - - - - - - - - - - +-------------------------------+
|                               |Masking-key, if MASK set to 1  |
+-------------------------------+-------------------------------+
| Masking-key (continued)       |          Payload Data         |
+-------------------------------- - - - - - - - - - - - - - - - +
:                     Payload Data continued ...                :
+ - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - +
|                     Payload Data continued ...                |
+---------------------------------------------------------------+
`

var (
    max_room_limit              = 100
    max_conn_limit              = 100000
    enable_ack_rto              = 0
    enable_verbose_log          = 0

    max_detect_peer_interval    = -1
    max_ping_interval           = -1

    client_ip_limit             = "0.0.0.0"
    console_ip_limit            = "127.0.0.1"

    app_server_ip               = "127.0.0.1"
	pid_file                    = "log/skytime.pid"
	skt_conf                    = flag.String("c", "conf/config.ini", "set configuration file for skytime")
)

func xheartbeat(ch <-chan time.Time) {

    for t := range ch {
        hrooms.cron_detect_all_peers()
        if enable_verbose_log > 0 {
        //  glog.Infof("application running since %d minutes", time.Now().Sub(StartTime) / 60)
            glog.Infof("handle detect peers background job take %d seconds", time.Now().Unix()-t.Unix())
        }
    }
}

func httpHandler(w http.ResponseWriter, r *http.Request) {

    is_ok := check_client_ipf( r.RemoteAddr )
    if !is_ok {
		http.Error(w, r.RemoteAddr + " is not allow to access skytime", 909)
        return
    }

	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported!", http.StatusInternalServerError)
		return
	}

	/**
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	*/

	if r.URL.Path == "/push" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Println(err)
		}

		go func() {
			time.Sleep(100 * time.Millisecond)
			hrooms.broadcast_Q <- body
		}()

		f.Flush()
		return
	}
}

// handle websocket requests from the peer.
func websocketHandler(w http.ResponseWriter, r *http.Request) {

    is_ok := check_client_ipf( r.RemoteAddr )
    if !is_ok {
		http.Error(w, r.RemoteAddr + " is not allow to access skytime", 909)
        return
    }

    peer_id := r.Header.Get("peer_id")

    /** fix later
    if len( peer_id ) != 32 {
		http.Error(w, "peer_id is invalid", 910)
        return
    }
    */

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		glog.Errorf("error %s", err)
		return
	}

    if hrooms.stats.total_peer_cnt > int64(max_conn_limit) {
		glog.Errorf("sorry, all the rooms are full")
        ws.Close()
        return
    }

	c := &connection{
		send: make(chan *push_msg_t, 4096),
		ws: ws,
		peer_id: peer_id,
		recv_ack_seq: make(chan int),
		write_msg_Q: make(map[int]*write_msg_t),
        notify_mu: make(chan bool, 1),
	}
    c.notify_mu <- true

	hrooms.register(c)

	if enable_ack_rto > 0 {
		go c.writeRTO()
	} else {
		go c.writePump()
	}

	go c.checkPongMessage()

}

func telnetHandler(c *net.TCPConn) {

    is_ok := check_client_ipf( c.RemoteAddr().String() )
    if !is_ok {
        return
    }

	defer c.Close()
	fmt.Printf("Connection from %s to %s established.\n", c.RemoteAddr(), c.LocalAddr())
	io.WriteString(c, fmt.Sprintf("Welcome on %s\n", c.LocalAddr()))
	buf := make([]byte, 4096)
	for {
		n, err := c.Read(buf)
		if (err != nil) || (n == 0) {
			c.Close()
			break
		}

		go func() {
			time.Sleep(100 * time.Millisecond)
			hrooms.broadcast_Q <- buf[0:n]
		}()

		io.WriteString(c, "sent to "+strconv.FormatInt(hrooms.stats.total_peer_cnt, 10)+" peers\n")
	}

	time.Sleep(150 * time.Millisecond)
	fmt.Printf("Connection from %v closed.\n", c.RemoteAddr())
	c.Close()
	return
}

func listenForTelnet(ln *net.TCPListener) {

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go telnetHandler(conn)
	}
}

/////////////////////////////////////////////////////////////////////////////////
// go calls init on start
func init_skytime_conf() {
	loadConfig(true)
	s := make(chan os.Signal, 1)
	//	signal.Notify(s, syscall.SIGUSR2)
	signal.Notify(s, syscall.SIGHUP)

	go func() {
		for {
			<-s
			loadConfig(false)
			glog.Info("Reloaded")
		}
	}()
}

/////////////////////////////////////////////////////////////////////////
var (
	StartTime time.Time
	pidFile   string
    client_ipf IPFilter
    console_ipf IPFilter
)

func check_console_ipf( remoteAddr string ) bool {

    if strings.Compare(console_ip_limit, "0.0.0.0") == 0 {
        return true
    }

    pos := strings.LastIndex(remoteAddr, ":")
    remote_peer_ip := remoteAddr[:pos]
    if !console_ipf.FilterIPString( remote_peer_ip ) {
        if enable_verbose_log > 0 {
		    glog.Infof("console %s is not allow to access skytime", remoteAddr)
        }
        return false
    }

    return true
}

func check_client_ipf( remoteAddr string ) bool {

    if strings.Compare(client_ip_limit, "0.0.0.0") == 0 {
        return true
    }

    pos := strings.LastIndex(remoteAddr, ":")
    remote_peer_ip := remoteAddr[:pos]
    if !client_ipf.FilterIPString( remote_peer_ip ) {
        if enable_verbose_log > 0 {
		    glog.Infof("client %s is not allow to access skytime\n", remoteAddr)
        }
        return false
    }

    return true
}

func main() {

    fmt.Print(ws_banner)
    fmt.Printf("Welcome to the SkyTime\n")
    fmt.Printf("Version:1.0.5.37\n")
	fmt.Printf("SkyTime started at: %s\n", time.Now().Format("2006-01-02 15:04:05"))

    StartTime = time.Now()
	runtime.GOMAXPROCS(runtime.NumCPU())

    ////////////// loading skytime config ///////////////////////////
    exec_file, _ := exec.LookPath(os.Args[0])
    exec_path, _ := filepath.Abs(exec_file)
    *skt_conf = filepath.Dir(exec_path) + "/" + *skt_conf
    fmt.Printf("loading config file of skytime is %s\n", *skt_conf)

    if _, err := os.Stat(*skt_conf); os.IsNotExist(err) {
        fmt.Printf("config file of skytime is not exist\n", *skt_conf)
        os.Exit(0)
    }

    var config *cfg.Cfg
    config, err := InitConfigFromFile( *skt_conf )
    if err != nil {
        fmt.Printf("failed to open config file of skytime\n")
        os.Exit(0)
    }

    if err := config.Load(); err != nil {
        fmt.Printf("config file of skytime is not correct\n")
        os.Exit(0)
    }
    ///////// enable verbose logging  //////////////////////////////
    verbose_log, err := config.ReadInt("enable_verbose_log", 0)
    if err != nil {
        fmt.Printf("%v", err)
    }
    if verbose_log > 0 {
	    flag.Set("log_dir", "log")
        flag.Set("alsologtostderr", "true")
    }
	flag.Set("v", "3")
	flag.Parse()

    enable_verbose_log = verbose_log
    glog.Infof("enable_verbose_log is %d", enable_verbose_log)
    ///////////////////////////////////////////////////////////////
	defer func() {
		if err := recover(); err != nil {
			var st = func(all bool) string {
				// Reserve 1K buffer at first
				buf := make([]byte, 512)

				for {
					size := runtime.Stack(buf, all)
					// The size of the buffer may be not enough to hold the stacktrace,
					// so double the buffer size
					if size == len(buf) {
						buf = make([]byte, len(buf)<<1)
						continue
					}
					break
				}

				return string(buf)
			}
			glog.Infof("panic: %s", err)
			glog.Infof("stack: %s", st(false))
		}
	}()
    ////////////////////////////////////////////////////////////////
    if err := createPidFile( pid_file ); err != nil {
		glog.Fatalf("create pidfile failed: %s", err)
	}
    glog.Infof("create pidfile of skytime is %s", pid_file)
    ////////////////////////////////////////////////////////////////
    conn_limit, err := config.ReadInt("max_conn_limit", 100000)
    if err != nil {
       glog.Infof("%v", err)
    }
    max_conn_limit = conn_limit
    glog.Infof("max_conn_limit is %d", max_conn_limit)
    ////////////////////////////////////////////////////////////////
    ips_limit, err := config.ReadString("client_ip_limit", "127.0.0.1")
    if err != nil {
       glog.Infof("%v", err)
    }
    client_ip_limit = ips_limit
    glog.Infof("client_ip_limit is %s", client_ip_limit)

    var data = []byte(ips_limit)
    if err := client_ipf.Load( data ); err != nil {
        glog.Infof("load client ip filter list failed %v", err)
    }
    ////////////////////////////////////////////////////////////////////////
    console_limit, err := config.ReadString("console_ip_limit", "127.0.0.1")
    if err != nil {
       glog.Infof("%v", err)
    }
    console_ip_limit = console_limit;
    glog.Infof("console_ip_limit is %s", console_ip_limit)

    var datax = []byte(console_limit)
    if err := console_ipf.Load( datax ); err != nil {
        glog.Infof("load console ip filter list failed %v", err)
    }
    ////////////////////////////////////////////////////////////////////////
    local_addrs, err := net.InterfaceAddrs()
    if err != nil {
        os.Stderr.WriteString("Oops:" + err.Error())
        os.Exit(1)
    }
    for _, la := range local_addrs {
        if ipnet, ok := la.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
            if ipnet.IP.To4() != nil {
                client_ipf.AddIPString( ipnet.IP.String() )
                glog.Infof("add local internal addr %s to client_ip_limit", ipnet.IP.String() )

                console_ipf.AddIPString( ipnet.IP.String() )
                glog.Infof("add local internal addr %s to console_ip_limit", ipnet.IP.String() )
            }
        }
    }

    glog.Infof("client_ip_limit is %s", client_ipf.String())
    glog.Infof("console_ip_limit is %s", console_ipf.String())
    /////////////////////////////////////////////////////////////////
    server_ip, err := config.ReadString("app_server_ip", "127.0.0.1")
    if err != nil {
       glog.Infof("%v", err)
    }
    app_server_ip = server_ip
    glog.Infof("app server ip is %s", app_server_ip)
	////////////////// init hub rooms ///////////////////////////////
	hrooms.run( config )
	go hrooms.broadcast()
    /////////////////////////////////////////////////////////////////
    ack_rto, err := config.ReadInt("enable_ack_rto", 0)
    if err != nil {
       glog.Infof("%v", err)
    }
    enable_ack_rto = ack_rto
    glog.Infof("enable_ack_rto is %d", enable_ack_rto)
	if enable_ack_rto > 0 {
		go hrooms.msg_ack_stat()
        go hrooms.processNotifyMessage()
	}
	///////////////////////skytime api router///////////////////////
	router := mux.NewRouter()
	router.HandleFunc("/push", httpHandler)
	router.HandleFunc("/ws", websocketHandler)
    router.HandleFunc("/api/heartbeat", apiHeartbeat)
	router.HandleFunc("/api/overview", apiOverview)
	router.HandleFunc("/api/server_rooms", apiGetServerGroup)
	router.HandleFunc("/api/broadcast_all", apiBroadcastAll)
	router.HandleFunc("/api/broadcast_range", apiBroadcastRange)
	router.HandleFunc("/api/broadcast_set", apiBroadcastSet)
	router.HandleFunc("/api/broadcast_peers", apiBroadcastPeers)
	router.HandleFunc("/api/room/{room_[0-9]+}", apiRoomview)
    /////////////////// new api for skylar  /////////////////////
	router.HandleFunc("/api/peers_is_online", apiPeersIsOnline)
	router.HandleFunc("/api/sync_broadcast_peers", apiSyncBroadcastPeers)
	////////////// telnet skytime ///////////////////////////////
    ln, err := net.ListenTCP("tcp", &net.TCPAddr{
        Port: 8001,
    })
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    go listenForTelnet(ln)
    ////////////////system signal handler///////////////////
    sc := make(chan os.Signal, 1)
    signal.Notify(sc,
    syscall.SIGHUP,
    syscall.SIGINT,
    syscall.SIGTERM,
    syscall.SIGQUIT)
    go func() {
        sig := <-sc
        glog.Infof("Got signal [%d] to exit.", sig)
        os.Exit(0)
    }()
    ////////////////////////////////////////////////////////
    ng := negroni.Classic()
    ////////////////////////////////////////////////////////
    enable_web_console, err := config.ReadInt("enable_web_console", 0)
    if err != nil {
        glog.Infof("%v", err)
    }
    glog.Infof("enable_web_console is %d", enable_web_console)
    if enable_web_console > 0 {
	    //http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
        glog.Infof("%s", filepath.Dir(exec_path) + "/static")
        ng.Use(negroni.NewStatic(http.Dir(filepath.Dir(exec_path) + "/static")))
    }
    ng.UseHandler(router)
    ////////////////////////////////////////////////////////////////
    listen_addr, err := config.ReadString("listen_addr", "127.0.0.1:8000")
    if err != nil {
        glog.Infof("%v", err)
    }
    glog.Infof("pid %d of SkyTime is running on %s", os.Getpid(), listen_addr)
    ////////////////////////////////////////////////////////////////
    enable_graceful, err := config.ReadInt("enable_graceful", 0)
    if err != nil {
        glog.Infof("%v", err)
    }
    glog.Infof("enable_graceful is %d", enable_graceful)
    if enable_graceful > 0 {
        go graceful.Run( listen_addr, 20 * time.Second, ng)
    } else {
        go ng.Run( listen_addr )
    }

    //////////////////// handler for heartbeat //////////////////////
    detect_peer_interval, err := config.ReadInt("detect_peer_interval", -1)
    if err != nil {
        glog.Infof("%v", err)
    }
    max_detect_peer_interval = detect_peer_interval
    glog.Infof("max_detect_peer_interval is %d", max_detect_peer_interval)

    if max_detect_peer_interval > 0 {
        xticker := time.NewTicker( time.Duration( max_detect_peer_interval ) * time.Second )
        defer xticker.Stop()
        go xheartbeat(xticker.C)
    }
    /////////////////////////////////////////////////////////////////

    var input string
    for input != "exit" {
        _, _ = fmt.Scanf("%v", &input)
        if input != "exit" {
            switch input {
            case "", "0", "5", "help", "info":
                fmt.Print("you can type \n1: \"exit\" to kill this application")
                fmt.Print("\n2: \"stats\" to show the amount of connected peers")
                fmt.Print("\n3: \"system\" to show info about the server")
                fmt.Print("\n4: \"time\" to show since when this application is running")
                fmt.Print("\n5: \"help\" to show this information")
                fmt.Println()

            case "1", "exit", "kill":
                fmt.Println("application get killed in 5 seconds")
                input = "exit"
                time.Sleep(5 * time.Second)

            case "2", "stats":
                hrooms.stat()

            case "3", "system":
                fmt.Printf("CPU cores: %d\nGo calls: %d\nGo routines: %d\nGo version: %v\nProcess ID: %v\n",
                runtime.NumCPU(),
                runtime.NumCgoCall(),
                runtime.NumGoroutine(),
                runtime.Version(),
                syscall.Getpid())

            case "4", "time":
                fmt.Printf("application running since %d minutes\n", (time.Now().Unix()-StartTime.Unix())/60)
            }
        }
    }

    removePidFile( pid_file )
	glog.Infof("Server on %s stopped", listen_addr)
    glog.Flush()

    os.Exit(0)
}
