var skytimeControllers = angular.module('skytimeControllers', ['ui.bootstrap', 'ngResource', 'highcharts-ng']);

skytimeControllers.config(['$interpolateProvider',
    function($interpolateProvider) {
        $interpolateProvider.startSymbol('[[');
        $interpolateProvider.endSymbol(']]');
    }
]);

skytimeControllers.config(['$httpProvider', function($httpProvider) {
    $httpProvider.defaults.useXDomain = true;
    delete $httpProvider.defaults.headers.common['X-Requested-With'];
}]);

skytimeControllers.factory('ServerRoomsFactory', ['$resource', function ($resource) {
    return $resource('/api/server_rooms', {}, {
        query: { method: 'GET', isArray: true,
            headers: { 'X-Auth-Key': '12345', 'X-Auth-Secret': 'secret' }
        },
        create : { method: 'PUT' }
    });
}]);

skytimeControllers.factory('BroadcastFactory', ['$resource', function ($resource) {
    return $resource('/api/broadcast_all', {}, {
        query: { method: 'GET', isArray: true },
        broadcast  : { method:'POST', url:'/api/broadcast_all'},
        broadPeers : { method:'POST', url:'/api/broadcast_peers'},
    });
}]);

skytimeControllers.factory('SlotFactory', ['$resource', function ($resource) {
    return $resource('/api/broadcast_range', {}, {
        rangeSet: { method: 'POST' },
        broadSet: { method:'POST', url:'/api/broadcast_set'},
        viewRoom: { method:'GET', url:'/api/room/:room_id', params : { room_id : '@room_id' },  isArray: true },
    });
}]);

skytimeControllers.controller('skytimeOverviewCtl', ['$scope', '$http', '$timeout',
function($scope, $http, $timeout) {

    var refreshChart = function(data) {
      var seriesArray = $scope.chartOps.series[0].data;

      if (seriesArray.length > 20) {
          seriesArray.shift();
      }
      seriesArray.push({
        x : new Date(),
        y : data
      });
      $scope.chartOps.series[0].data = seriesArray;
    }

    $scope.refresh = function() {

      var custom_headers = {
          headers:  {
              'X-Auth-Key': '12345',
              'X-Auth-Secret': 'secret'
          }
      };
      $http.get('/api/overview', custom_headers).success(function(succData) {

			$scope.product              = succData['product'];
			$scope.hub_room_cnt         = succData['hub_room_cnt'];
			$scope.online_peer_cnt      = succData['online_peer_cnt'];
			$scope.total_br_msg_cnt     = succData['total_br_msg_cnt'];
			$scope.total_br_msg_size    = succData['total_br_msg_size'];
			$scope.br_msg_avg_time      = succData['br_msg_avg_time'];

			$scope.cpu_cores            = succData['cpu_cores'];
			$scope.go_routines          = succData['go_routines'];
			$scope.go_version           = succData['go_version'];
			$scope.process_id           = succData['process_id'];
			$scope.ops                  = succData['ops'];

			if (succData['ops'] !== undefined && succData['ops'] >= 0) {
				$scope.ops = succData['ops'];
			} else {
				$scope.ops = 0;
			}
			refreshChart($scope.ops)
		});
	}

    $scope.refresh();

    $scope.chartOps = {
        options: {
            global: {
                useUTC: false,
            },
            chart: {
                useUTC: false,
                type: 'spline',
            }
        },
        series: [{
            name: 'OP/s',
            data: []
        }],
        title: {
            text: 'OP/s'
        },
        xAxis: {
            type : "datetime",
            title: {
                text: 'Time'
            },
        },
        yAxis: {
            title: {
                text: 'value'
            },

        },
    };

    (function autoUpdate() {
        $timeout(autoUpdate, 3000);
    	$scope.refresh();
    }());

}]);


skytimeControllers.controller('skytimeServerGroupMainCtl', ['$scope', '$http', '$modal', '$log', 'ServerRoomsFactory', 'BroadcastFactory', 'SlotFactory',
function($scope, $http, $modal, $log, ServerRoomsFactory, BroadcastFactory, SlotFactory) {

	$scope.checkRoom = function(room) {
        var modalInstance = $modal.open({
            templateUrl: 'slotInfoModal',
            controller: ['$scope', '$modalInstance', function($scope, $modalInstance) {
				SlotFactory.viewRoom(room,  function(succData){
					$scope.peers = succData;
				}, function(failedData) {
					alert(failedData.data);
				});

                $scope.ok = function (room) {
                	$modalInstance.close(room);
                };
                $scope.cancel = function() {
                   $modalInstance.close(null);
                }
            }],
			windowClass: 'large-Modal',
            size: 'sm',
        });

        modalInstance.result.then(function (room) {
            if (room) {}
        });
    }

	$scope.broadPeers = function(groupId) {
        var modalInstance = $modal.open({
            templateUrl: 'broadPeersModal',
            controller: ['$scope', '$modalInstance', function($scope, $modalInstance) {
                  $scope.task = {'peers': '', 'br_msg': ""};
                  $scope.ok = function (task) {
                      $modalInstance.close(task);
                  };
                  $scope.cancel = function() {
                      $modalInstance.close(null);
                  }
            }],
			      windowClass: 'large-Modal',
            size: 'sm',
        });

        modalInstance.result.then(function (task) {
            if (task) {
                BroadcastFactory.broadPeers(task, function(succData){
                    alert("success")
                }, function(failedData) {
                    alert(failedData.data);
                });
            }
        });
    }

    $scope.broadcastAllRoom = function() {
        var modalInstance = $modal.open({
            templateUrl: 'broadcastAllModal',
            controller: ['$scope', '$modalInstance', function ($scope, $modalInstance) {
                  $scope.ok = function (msg) {
                      $modalInstance.close(msg);
                  };
                  $scope.cancel = function() {
                      $modalInstance.close(null);
                  }
            }],
			windowClass: 'large-Modal',
            size: 'sm',
        });

        modalInstance.result.then(function (msg) {
            if (msg) {
                BroadcastFactory.broadcast(msg, function(succData) {
                    alert("success")
                }, function(failedData) {
                    alert(failedData.data);
                })
            }
        });
    }

	$scope.rangeSet = function() {
        var modalInstance = $modal.open({
            templateUrl: 'broadcastRangeSetModal',
            controller: ['$scope', '$modalInstance', function($scope, $modalInstance) {
                $scope.task = {'from': '-1', 'to': '-1', 'msg': ''};

                $scope.ok = function (task) {
                    $modalInstance.close(task);
                };

                $scope.cancel = function() {
                    $modalInstance.close(null);
                }
            }],
			windowClass: 'large-Modal',
            size: 'sm',
        });

        modalInstance.result.then(function (task) {
            if (task) {
                SlotFactory.rangeSet(task, function() {
                    alert("success")
                }, function(failedData) {
                    alert(failedData.data)
                })
            }
        });
    }

	$scope.broadSet = function() {
		var modalInstance = $modal.open({
			templateUrl: 'broadcastSetModal',
			controller: ['$scope', '$modalInstance', function($scope, $modalInstance) {
				$scope.task = {'from': '', 'msg': '' };
				$scope.ok = function (task) {
					$modalInstance.close(task);
				};
				$scope.cancel = function() {
					$modalInstance.close(null);
				}
			}],
			windowClass: 'large-Modal',
			size: 'sm',
		});

		modalInstance.result.then(function (task) {
			if (task) {
				SlotFactory.broadSet(task, function() {
                    alert("success")
				}, function(failedData) {
					alert(failedData.data)
				})
			}
		});
	}

    $scope.refresh = function() {
        $scope.server_rooms = ServerRoomsFactory.query();
    }

    // query server group
    $scope.server_rooms = ServerRoomsFactory.query();

}]);
