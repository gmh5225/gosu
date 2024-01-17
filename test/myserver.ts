import * as http from "node:http";
import { WebSocketServer } from "ws";

const server = http.createServer((req, res) => {
	if (req.url === "/" && req.method === "GET") {
		let src = `Hello! ${process.env.GOSU_NS} ${process.env.GOSU_LOCAL}`;
		for (const header of Object.keys(req.headers)) {
			src += `\n${header}: ${req.headers[header]}`;
		}
		res.end(src);

		return;
	}
});

const wss = new WebSocketServer({ server });
wss.on("connection", function connection(ws) {
	ws.on("error", console.error);
	ws.on("message", function message(data, isBinary) {
		console.log({ type: "message", data, isBinary });
	});
});

const id = parseInt(process.env.GOSU_CID || "0");
console.log(`Server ${id} is running...`, process.env.GOSU_SERVE);
/*

server.listen(8000).on("listening", () => {
	console.log({ type: "ready", adr: server.address() });
});
*/
server.listen(process.env.GOSU_SERVE).on("listening", () => {
	console.log({ type: "ready", adr: server.address() });
});
