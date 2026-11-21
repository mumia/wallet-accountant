// Runs in the mcr.microsoft.com/playwright container (make local-browser-test),
// on the k3d cluster's Docker network. Chromium resolves every *.localhost to
// 127.0.0.1 itself, so global-setup.js forwards 127.0.0.1:<the URL's port>
// to the cluster's ingress (WA_INGRESS), and the browser keeps the real
// origins: https://wallet.localhost:18443 and https://auth.localhost:18443.
const { defineConfig, devices } = require("@playwright/test");
const { execFileSync } = require("node:child_process");

// Certificates verified against the mkcert CA (tls-pin.js).
const pins = execFileSync(process.execPath, [require.resolve("./tls-pin.js")], { encoding: "utf8" });

module.exports = defineConfig({
	testDir: ".",
	globalSetup: require.resolve("./global-setup.js"),
	outputDir: process.env.WA_RESULTS_DIR || "test-results",
	reporter: [["list"]],
	timeout: 120_000,
	expect: { timeout: 15_000 },
	retries: 0,
	workers: 1,
	use: {
		headless: true,
		trace: "retain-on-failure",
		screenshot: "only-on-failure",
	},
	projects: [
		{
			name: "chromium",
			use: {
				...devices["Desktop Chrome"],
				launchOptions: { args: [`--ignore-certificate-errors-spki-list=${pins}`] },
			},
		},
	],
});
