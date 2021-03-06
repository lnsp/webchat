var protocol = "ws://"
if (location.protocol === "https:") {
    protocol = "wss://"
}
var server = protocol + location.host + "/chat/";
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
        app.addMessage(JSON.parse(event.data));
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
    },
    methods: {
        addMessage: function (msg) {
            this.messages.push(msg);
            if (this.messages.length > 50) {
                this.messages.splice(0, this.messages.length - 20);
            }
            window.setTimeout(this.scrollToEnd, 10);
        },
        scrollToEnd: function () {
            var container = this.$el.querySelector(".chat-history");
            container.scrollTop = container.scrollHeight;
        },
    },
});