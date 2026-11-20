// Prints Chromium's --ignore-certificate-errors-spki-list value for the
// certificates the cluster's ingress serves for each name in WA_TLS_NAMES,
// after checking each one chains to the mkcert CA (WA_CA_FILE) and names that
// host. The Playwright image has no certutil to add the CA to Chromium's NSS
// store, so the browser trusts exactly these verified certificates instead.
//
// WA_INGRESS is host:port of the ingress (the k3d load balancer's 443).
const fs = require("node:fs");
const tls = require("node:tls");
const crypto = require("node:crypto");

const [host, port] = (process.env.WA_INGRESS || "").split(":");
const ca = fs.readFileSync(process.env.WA_CA_FILE);
const names = (process.env.WA_TLS_NAMES || "").split(/\s+/).filter(Boolean);
if (!host || !port || names.length === 0) {
	console.error("tls-pin: set WA_INGRESS (host:port), WA_CA_FILE and WA_TLS_NAMES");
	process.exit(2);
}

function pin(name) {
	return new Promise((resolve, reject) => {
		const socket = tls.connect({ host, port: Number(port), servername: name, ca }, () => {
			const cert = socket.getPeerCertificate();
			socket.end();
			const spki = new crypto.X509Certificate(cert.raw).publicKey.export({ type: "spki", format: "der" });
			resolve(crypto.createHash("sha256").update(spki).digest("base64"));
		});
		socket.on("error", (err) => reject(new Error(`${name}: ${err.message}`)));
	});
}

Promise.all(names.map(pin)).then(
	(pins) => process.stdout.write([...new Set(pins)].join(",")),
	(err) => {
		console.error(`tls-pin: ${err.message}`);
		process.exit(1);
	},
);
