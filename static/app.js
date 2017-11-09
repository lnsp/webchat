var server = "ws://" + location.host + "/chat/";
var socket = null;
window.onload = function () {
    socket = new WebSocket(server);
    socket.onopen = function () {
        console.log("connected to " + server);
    }
    socket.onclose = function (e) {
        console.log("connection closed (" + e.code + ")");
    }
    socket.onmessage = function (event) {
        var message = JSON.parse(event.data);
        app.messages.push(message);

        if (app.messages.length > 20) {
            app.messages.splice(0, app.messages.length - 20);
        }
    }
}

function send() {
    var input = document.getElementById('message');
    var msg = input.value;
    if (msg === "") return false;
    input.value = '';
    input.focus();
    socket.send(msg);
    return false;
}

var app = new Vue({
    el: "#app",
    data: {
        messages: [],
    }
});