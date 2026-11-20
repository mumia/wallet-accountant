// Before the tests: checks the vendored oidc-client-ts bundle is the version
// package.json names, then forwards 127.0.0.1:<WA_UI_BASE_URL's port> to the
// cluster's ingress (WA_INGRESS) for as long as the tests run.
const fs = require("node:fs");
const net = require("node:net");
const path = require("node:path");

module.exports = async () => {
	const vendored = path.join(__dirname, "../site/vendor/oidc-client-ts.min.js");
	const packaged = path.join(__dirname, "node_modules/oidc-client-ts/dist/browser/oidc-client-ts.min.js");
	if (!fs.readFileSync(vendored).equals(fs.readFileSync(packaged))) {
		throw new Error("site/vendor/oidc-client-ts.min.js is not the version e2e/package.json names: run make stand-in-ui-vendor");
	}

	const [host, port] = process.env.WA_INGRESS.split(":");
	const listenPort = Number(new URL(process.env.WA_UI_BASE_URL).port || 443);
	const sockets = new Set();
	const server = net.createServer((client) => {
		const upstream = net.connect(Number(port), host);
		for (const s of [client, upstream]) {
			sockets.add(s);
			s.on("close", () => sockets.delete(s));
		}
		client.pipe(upstream).pipe(client);
		client.on("error", () => upstream.destroy());
		upstream.on("error", () => client.destroy());
	});
	await new Promise((resolve, reject) => {
		server.once("error", reject);
		server.listen(listenPort, "127.0.0.1", resolve);
	});
	return () =>
		new Promise((resolve) => {
			server.close(resolve);
			for (const s of sockets) s.destroy();
		});
};
