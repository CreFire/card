extends Node

const NetworkClient = preload("res://scripts/net/network_client.gd")

var network: NetworkClient


func _ready() -> void:
    network = NetworkClient.new()
    log_info("autoload app initialized")


func _process(_delta: float) -> void:
    if network != null:
        network.poll()


func log_info(message: String) -> void:
    print("[App] %s" % message)
