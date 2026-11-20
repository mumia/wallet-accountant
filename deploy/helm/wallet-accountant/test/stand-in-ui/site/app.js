// The stand-in UI's one page. It reads /config.json (docs/architecture/ui-contract.md),
// signs in through the `web` app with oidc-client-ts (authorization code with
// PKCE, public client), and shows /api/me's answer. "Check session" asks
// Zitadel for a sign-in with prompt=none, which shows whether the Zitadel
// session is still alive.
"use strict";

const $ = (id) => document.querySelector(`[data-testid="${id}"]`);
const button = (id) => document.getElementById(id);

async function main() {
	const config = await (await fetch("/config.json", { cache: "no-store" })).json();
	const origin = window.location.origin;
	const manager = new oidc.UserManager({
		authority: config.oidc.issuer,
		client_id: config.oidc.clientId,
		redirect_uri: origin + config.oidc.redirectPath,
		post_logout_redirect_uri: origin + config.oidc.postLogoutRedirectPath,
		response_type: "code",
		scope: "openid profile offline_access",
		userStore: new oidc.WebStorageStateStore({ store: window.sessionStorage }),
	});

	const url = new URL(window.location.href);
	if (url.pathname === config.oidc.redirectPath) {
		try {
			await manager.signinRedirectCallback();
			if (sessionStorage.getItem("checking-session")) $("session").textContent = "active";
		} catch (err) {
			// A prompt=none sign-in without a Zitadel session ends here.
			$("session").textContent = err.error || String(err);
		}
		sessionStorage.removeItem("checking-session");
		window.history.replaceState({}, "", "/");
	} else if (url.pathname === config.oidc.postLogoutRedirectPath && url.searchParams.has("state")) {
		await manager.signoutRedirectCallback().catch(() => {});
		window.history.replaceState({}, "", "/");
	}

	button("sign-in").onclick = () => manager.signinRedirect();
	button("sign-out").onclick = () => manager.signoutRedirect();
	button("check-session").onclick = () => {
		sessionStorage.setItem("checking-session", "1");
		manager.signinRedirect({ prompt: "none" });
	};
	for (const id of ["sign-in", "sign-out", "check-session"]) button(id).disabled = false;

	const user = await manager.getUser();
	if (!user || user.expired) {
		$("status").textContent = "signed out";
		return;
	}
	$("status").textContent = "signed in";
	const response = await fetch(`${config.apiBaseUrl}/me`, {
		headers: { Authorization: `Bearer ${user.access_token}` },
	});
	$("me-status").textContent = String(response.status);
	$("me").textContent = await response.text();
}

main().catch((err) => {
	$("error").textContent = String(err && err.stack ? err.stack : err);
});
