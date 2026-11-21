// The stand-in UI's browser test (spec "Testing Decisions"): a headless
// Chromium drives the stand-in UI on https://wallet.localhost:<port>, signs in
// through Zitadel with the `web` app (authorization code with PKCE), and
// checks what the real UI will rely on (docs/architecture/ui-contract.md):
//
// - the image meets the contract (/healthz, SPA fallback, config.json);
// - /api goes to the app on the same origin;
// - the access token the page sends to /api/me carries the API's audience,
//   and the app accepts it;
// - signing out ends the Zitadel session, not only the page's.
//
// WA_UI_USER and WA_UI_PASSWORD are a Zitadel human user's login name and
// password (deploy/local/smoke.sh creates one); WA_UI_BASE_URL is the UI's
// origin and WA_AUDIENCE the API's audience.
const { test, expect } = require("@playwright/test");

const base = process.env.WA_UI_BASE_URL;
const user = process.env.WA_UI_USER;
const password = process.env.WA_UI_PASSWORD;
const audience = process.env.WA_AUDIENCE || "wallet-accountant";

test.beforeAll(() => {
	expect(base, "WA_UI_BASE_URL").toBeTruthy();
	expect(user, "WA_UI_USER").toBeTruthy();
	expect(password, "WA_UI_PASSWORD").toBeTruthy();
});

// The access token's payload, decoded (not verified: the app verifies it).
function claims(authorization) {
	const token = (authorization || "").replace(/^Bearer /, "");
	const payload = token.split(".")[1] || "";
	return JSON.parse(Buffer.from(payload, "base64url").toString("utf8") || "{}");
}

// Zitadel's v1 login: login name, then password. Its optional prompts (set up
// a second factor, for instance) are skipped.
async function signInAtZitadel(page) {
	const issuer = new URL(JSON.parse(await (await page.request.get(`${base}/config.json`)).text()).oidc.issuer);
	await page.waitForURL((url) => url.host === issuer.host);
	await page.locator("#loginName").fill(user);
	await page.locator("#submit-button").click();
	await page.locator("#password").fill(password);
	await page.locator("#submit-button").click();
	const skip = page.getByRole("button", { name: "Skip" });
	const left = page.waitForURL((url) => url.host !== issuer.host);
	await Promise.race([left, skip.waitFor()]);
	if (new URL(page.url()).host === issuer.host) await skip.click();
	await left;
}

test("the stand-in UI meets the UI contract, and /api goes to the app on the same origin", async ({ request }) => {
	const health = await request.get(`${base}/healthz`);
	expect(health.status()).toBe(200);

	const deepLink = await request.get(`${base}/households/some-id/accounts`);
	expect(deepLink.status()).toBe(200);
	expect(await deepLink.text()).toContain('data-testid="stand-in-ui"');

	const config = await request.get(`${base}/config.json`);
	expect(config.status()).toBe(200);
	expect(config.headers()["cache-control"]).toContain("no-cache");
	const { apiBaseUrl, oidc } = await config.json();
	expect(apiBaseUrl).toBe("/api");
	expect(oidc.clientId, "the setup Job's web client ID").toMatch(/^\d+$/);
	expect(oidc.audience).toBe(audience);

	const me = await request.get(`${base}/api/me`);
	expect(me.status(), "the app, not the UI, answers /api/me").toBe(401);
});

test("signs in through the web app, calls /api/me with the API's audience, and signing out ends the Zitadel session", async ({ page }) => {
	await page.goto(`${base}/`);
	await expect(page.getByTestId("status")).toHaveText("signed out");

	// Sign in: the code flow with PKCE, back to /auth/callback, then /api/me.
	const meRequest = page.waitForRequest((r) => new URL(r.url()).pathname === "/api/me");
	await page.getByRole("button", { name: "Sign in" }).click();
	await signInAtZitadel(page);
	await expect(page.getByTestId("status")).toHaveText("signed in");
	expect(new URL(page.url()).pathname, "back from /auth/callback").toBe("/");

	const sent = await meRequest;
	expect(new URL(sent.url()).origin, "same origin, no CORS").toBe(new URL(base).origin);
	const aud = [].concat(claims(sent.headers()["authorization"]).aud || []);
	expect(aud, "the access token's audience").toContain(audience);

	// The app accepted the token: a new identity is asked to register
	// (ADR-0017) rather than refused (401).
	await expect(page.getByTestId("me-status")).toHaveText("403");
	await expect(page.getByTestId("me")).toContainText("registration-required");

	// The Zitadel session is alive: a sign-in with prompt=none succeeds.
	await page.getByRole("button", { name: "Check session" }).click();
	await expect(page.getByTestId("session")).toHaveText("active");

	// Sign out through Zitadel's end_session, back to the post-logout URI.
	await page.getByRole("button", { name: "Sign out" }).click();
	await page.waitForURL((url) => url.origin === new URL(base).origin && url.pathname === "/");
	await expect(page.getByTestId("status")).toHaveText("signed out");

	// The Zitadel session ended: the same prompt=none sign-in now fails.
	// Zitadel (v4, v1 login) answers interaction_required, since the browser
	// still knows the user, who is signed out; login_required is as good.
	await page.getByRole("button", { name: "Check session" }).click();
	await expect(page.getByTestId("session")).toHaveText(/^(login_required|interaction_required)$/);
});
