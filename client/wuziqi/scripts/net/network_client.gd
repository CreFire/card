extends RefCounted
class_name NetworkClient

var endpoint: String = "ws://127.0.0.1:9004/gate/ws"
var socket: WebSocketPeer = WebSocketPeer.new()
var connect_token: String = ""
var session_id: String = ""
var conn_id: String = ""
var authenticated: bool = false
var auth_sent: bool = false


func connect_to_server() -> void:
    if connect_token.is_empty():
        App.log_info("connect token is empty, call apply_login_result first")
        return

    if socket.get_ready_state() == WebSocketPeer.STATE_OPEN:
        return

    authenticated = false
    auth_sent = false
    conn_id = ""
    socket = WebSocketPeer.new()
    var err := socket.connect_to_url(endpoint)
    if err != OK:
        App.log_info("connect websocket failed: %s" % err)


func apply_login_result(login_result: Dictionary) -> void:
    connect_token = str(login_result.get("connect_token", ""))
    session_id = str(login_result.get("session_id", ""))


func poll() -> void:
    if socket.get_ready_state() == WebSocketPeer.STATE_CLOSED:
        return

    socket.poll()

    if socket.get_ready_state() == WebSocketPeer.STATE_OPEN and not auth_sent and not connect_token.is_empty():
        _send_json({
            "op": "login.token",
            "token": connect_token,
        })
        auth_sent = true

    while socket.get_ready_state() == WebSocketPeer.STATE_OPEN and socket.get_available_packet_count() > 0:
        var packet := socket.get_packet().get_string_from_utf8()
        var payload = JSON.parse_string(packet)
        if typeof(payload) != TYPE_DICTIONARY:
            App.log_info("unexpected websocket packet: %s" % packet)
            continue

        if payload.has("conn_id"):
            conn_id = str(payload["conn_id"])
        if str(payload.get("code", "")) == "login_ok":
            authenticated = true
        App.log_info("gate message: %s" % JSON.stringify(payload))


func reconnect_by_session() -> void:
    if session_id.is_empty():
        App.log_info("session id is empty")
        return
    _send_json({
        "op": "login.session",
        "session_id": session_id,
    })


func send_ping() -> void:
    _send_json({"op": "ping"})


func _send_json(payload: Dictionary) -> void:
    if socket.get_ready_state() != WebSocketPeer.STATE_OPEN:
        return
    var err := socket.send_text(JSON.stringify(payload))
    if err != OK:
        App.log_info("send websocket payload failed: %s" % err)
