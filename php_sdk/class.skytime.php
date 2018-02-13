<?php

/**
 *  class     : class.skytime.php
 *	desc      : class skytime for long connection of skylar
 *  author    : yunlong.lee
 *  mail      : yunlong.lee@163.com
 */

const API_BROADCAST_ALL 	= "/api/broadcast_all";
const API_BROADCAST_PEERS 	= "/api/broadcast_peers";
const API_HEART_BEAT 		= "/api/heartbeat";

class skytime {

    private static $instance;

	public $listen_addr = "127.0.0.1";
	public $listen_port = "8000";

	public $enable_debug = 0;

    /**
     * Initialize the class so that the data is in a known state.
     */
 	public function __construct() {
    	$this->listen_port 		= 8000;
    	$this->listen_addr 		= "127.0.0.1";
 		$this->enable_debug 	= true;
  	}

    public static function get_instance() {

        if (!isset(self::$instance))
        {
            $c = __CLASS__;
            self::$instance = new $c;
        }
        return self::$instance;
    }

	public function init( $addr, $port, $debug = false ) {
    	$this->listen_addr 		= $addr;
 		$this->listen_port 		= $port;
 		$this->enable_debug 	= $debug;
	}

	/**
	 *  skytime is avaiable
	 *  @param string $msg 		broadcast message from application console
	 *	@param int 	  $timeout 	request timeout
	 */
	public function is_avaiable () {

		$api_uri = "http://".$this->listen_addr . ":" . $this->listen_port . API_HEART_BEAT;

    	$ch  = curl_init();
    	curl_setopt($ch, CURLOPT_URL, $api_uri);
    	curl_setopt($ch, CURLOPT_POST, 1);
    	curl_setopt($ch, CURLOPT_POSTFIELDS, "");
    	curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
    	curl_setopt($ch, CURLOPT_TIMEOUT, $timeout);

    	$json_data = curl_exec($ch);

        if( $this->enable_debug ) {

            print_r($json_data);
    	    $httpcode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
		    if(!curl_errno($ch)){
  			    $info = curl_getinfo($ch);
  			    echo 'Took ' . $info['total_time'] . ' seconds to send a request to ' . $info['url'] . "\n";
		    } else {
  			    echo 'Curl error: ' . curl_error($ch) . "\n";
		    }
        }

    	curl_close($ch);

    	$data = json_decode( $json_data, true );
    	if ( $data["status"] != "running" ) {
    		return false;
        }
    	return true;
	}


	/**
	 *  broadcast message to all peers
	 *  @param string $msg 		broadcast message from application console
	 *	@param int 	  $timeout 	request timeout
	 */
	public function broadcast_all( $msg, $timeout ) {
		if ( empty($msg) )
			return false;

		$api_uri = "http://".$this->listen_addr . ":" . $this->listen_port . API_BROADCAST_ALL;

    	$ch  = curl_init();
    	curl_setopt($ch, CURLOPT_URL, $api_uri);
    	curl_setopt($ch, CURLOPT_POST, 1);
    	curl_setopt($ch, CURLOPT_POSTFIELDS, $msg);
    	curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
    	curl_setopt($ch, CURLOPT_TIMEOUT, $timeout);

    	$json_data = curl_exec($ch);

        if( $this->enable_debug ) {
    	    $httpcode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
		    if(!curl_errno($ch)){
  			    $info = curl_getinfo($ch);
  			    echo 'Took ' . $info['total_time'] . ' seconds to send a request to ' . $info['url'] . "\n";
		    } else {
  			    echo 'Curl error: ' . curl_error($ch) . "\n";
		    }
        }

        curl_close($ch);

    	$data = json_decode( $json_data, true );
    	if ( $data[ "ret"] != 0 ) {
    		return false;
        }
    	return true;
	}

	/**
	 *  broadcast message to peers
	 *  @param string $msg 		broadcast message from application console
	 *	@param array  $peers 	peers of receiving broadcast message
	 *	@param int 	  $timeout 	request timeout
	 */
	public function broadcast_peers( $msg, $peers, $timeout ) {

		if ( empty( $msg ))
			return false;

		if ( !is_array($peers) )
			return false;

		$data = array(
			"peers"		=> implode(",", $peers),
			"br_msg"	=> $msg,
		);

		$api_uri = "http://".$this->listen_addr . ":" . $this->listen_port . API_BROADCAST_PEERS;

    	$ch  = curl_init();
    	curl_setopt($ch, CURLOPT_URL, $api_uri);
    	curl_setopt($ch, CURLOPT_POST, 1);
    	curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode($data));
    	curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
    	curl_setopt($ch, CURLOPT_TIMEOUT, $timeout);
    	$json_data = curl_exec($ch);

        if( $this->enable_debug ) {
    	    $httpcode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
		    if(!curl_errno($ch)){
  			    $info = curl_getinfo($ch);
  			    echo 'Took ' . $info['total_time'] . ' seconds to send a request to ' . $info['url'] . "\n";
		    } else {
  			    echo 'Curl error: ' . curl_error($ch) . "\n";
		    }
        }

    	curl_close($ch);

    	$data = json_decode( $json_data, true );
    	if ( $data[ "ret"] != 0 ) {
    		return false;
    	}

    	return true;
	}

}

?>
