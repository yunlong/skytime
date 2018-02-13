<?php
    require_once "class.skytime.php";

    $peers = array(
        "D1C17BBD-49BC-5A7B-BB60-4E90C02941D2",
        "C4F94125-C298-0437-B238-8987972B25B7",
    );

    $data = array(
        "peers"    => implode(",", $peers),
        "br_msg"   => "HELLO Skytime",
    );

    $skt = new skytime();
    $skt->init( "10.187.102.69", 8000 );
    if( $skt->is_avaiable() ) {
        echo "connect ok\n";
    } else {
        echo "connect failed\n";
    }

    $msg = "GGDD";
    $ret = $skt->broadcast_all( $msg, 3 );
    if ( $ret ) {
        echo "broadcast msg to all ok\n";
    } else {
        echo "broadcast msg to all failed\n";
    }

    $msg = "I LOVE YOU SKYTIME";
    $ret = $skt->broadcast_peers( $msg, $peers, 3);
    if ( $ret ) {
        echo "broadcast msg to peers ok\n";
    } else {
        echo "broadcast msg to all failed\n";
    }

?>
