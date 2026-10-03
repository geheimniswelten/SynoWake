# DSM integration

[Deutsch](ARCHITEKTUR.md) · English

## Files and processes

```text
SynoWake.spk
  INFO                       Package ID, apollolake, minimum DSM version
  conf/privilege             defaults.run-as: package only
  scripts/                   init/start/stop/status
  package.tgz
    bin/synowake             Service, Task Scheduler/lifecycle CLI
    ui/api.cgi               Same binary as an unprivileged gateway
    run/backend.sock         Local Unix socket created at runtime
    ui/index.html            Browser interface
    ui/app.js
    ui/scheduler.js          Browser TaskScheduler client
    ui/i18n.js               Language selection and text formatting
    ui/translations.js       Generated shared translation catalog
    ui/assets/synowake.css   Iframe-only styles scoped to #synowake-app
    ui/config                DSM application registration
    ui/SynoWake.js           Native DSM window embedding the interface
    ui/images/               Code-generated PNG icons
    ui/texts/                German/English notification templates
```

DSM serves `dsmuidir="ui"` at `/webman/3rdparty/SynoWake/` and executes the CGI for each request. The persistent package service receives forwarded requests over a Unix socket and opens no TCP listening port. The CGI never opens private package state. The start script launches the service as the package account and waits for health. Stop disables execution and shuts the service down using a private-key-protected control request. Status returns 0 when active/reachable and 3 otherwise. DSM remains responsible for timing.

Private state resides in the package `var` directory, outside the interface. File locking and atomic replacement protect concurrent requests and lifecycle commands. Administrator state includes each schedule's command and secret so the interface can verify and recover its DSM task association. The command also appears in DSM's task script. No separate secret field is exposed in interface state; secrets are not rendered in the DOM or written to execution logs.

## Languages and packet transmission

`internal/synowake/translations.json` stores German source keys and English translations. Go embeds it; the builder exports `ui/translations.js` and validates matching parameters. The interface prefers the DSM session language, then the browser language, with English for unsupported languages. The CGI forwards `Accept-Language`; only message fields in a response copy are localized. Names and persistent state remain unchanged. New log records additionally retain a template and separate arguments. Single-pass substitution preserves braces and markup characters in user names.

The Linux sender transmits three identical UDP packets with two pauses of 20 ms. The entire burst updates one durable execution record. A send failure stops the burst. Status checks never trigger another transmission; separate later status records remain possible. Lifecycle and Log Center output follow the package process language, defaulting to English. Technical operating-system details are retained verbatim.

## Local networks and favorites

Device `favorite` defaults to false when absent, preserving the existing state format. Protected POST `device-favorite` changes only this field atomically. Overview, status counters and quick selection use favorites; the device inventory and schedules use all devices.

State includes active IPv4 interfaces, NAS IP, CIDR, bounded search suggestion and broadcast. Loopback and IPv4 link-local addresses are excluded. Original CGI server metadata prioritizes the interface used to reach DSM; the request host is the fallback. Discovery refreshes these interfaces whenever opened.

Successful searches save normalized `discoveryCidr`, which is revalidated against current interfaces before reuse. Removed networks are replaced by a current suggestion. Imported DNS suggestions strip `.fritz.box`, `.local` and `.lan`; saved names do not change. HTML, CSS and modules use versioned URLs verified against INFO.

Normalized MAC matching identifies saved results, shows their existing names and blocks duplicate import. A refresh before import updates those markers while retaining edited names and selections for new results. The atomic save transaction additionally rejects new IDs with an existing MAC; existing devices remain editable by ID.

The entire search range must fit a connected interface. Address classification alone does not decide admission: a connected nonprivate `192.167.178.0/24` is allowed, while an unconnected private range is rejected. Suggestions on larger networks use the `/24` containing the NAS; `/25`–`/30` masks remain intact. Validation checks both network start and end. Device IP validation accepts private addresses and directly connected additional hosts. Additional nonprivate broadcast targets must match an active interface broadcast; discovery returns that broadcast for import. `255.255.255.255` remains available.

## Style isolation

The registered UI directory contains no reserved desktop `style.css`. The iframe explicitly loads `assets/synowake.css`. All selectors, including media queries and dialogs, begin with `#synowake-app`, which exists only on the iframe body. Animation names are package-specific. Browser regression tests intentionally load the private stylesheet in an outer document and compare tables, widgets and form dimensions/styles before and after. The builder rejects global `ui/style.css`.

## Task Scheduler

An authenticated administrator interface uses `SYNO.Core.TaskScheduler` through the parent DSM document's `SYNO.API.Request`. DSM serializes copied typed parameters and supplies the live token and dynamic `X-SYNO-HASH` protection. Passwords and hashes are never extracted, stored or reconstructed. Read-only inspection of the target NAS's own TaskScheduler/SDK scripts identified version 3 for native create/get/set. The package installs neither static `task_config.xml` tasks nor `/etc/crontab` content.

API discovery tries `entry.cgi`, then `query.cgi` for a missing path or 404. get/create/set consider versions 2–4, preferring advertised versions before known newer ones on outdated catalogs, while respecting the minimum. Successful versions are cached separately per method. list prefers version 2, matching DSM 7.1's native task grid, with version 3 as the known alternative; delete uses 2. Only explicit 104 version rejection allows a different write dispatch. Read-only list may additionally try its other known version after explicit method rejection 103. Successful versions remain cached separately per method. Permission errors, HTTP failures and ambiguous responses never trigger speculative retries. Interrupted writes retain prepared state for inventory reconciliation. The native call has a 22-second deadline; late responses cannot change its outcome. There is no raw retry after native failure. Direct WebAPI requests are used only when the parent SDK is unavailable beforehand and may lack DSM's additional session protection.

Reachability first uses ICMP. Local execution failures, including permission failure with exit code 1, trigger TCP probing. A connection or ECONNREFUSED proves an IP response; timeouts produce Unknown. TCP needs no elevated permissions and triggers ordinary kernel ARP resolution. Neighbor tables supply MAC addresses. No kernel parameters or system permissions are changed.

Scheduled tasks invoke the CLI with schedule ID, secret and a fixed local callback. The CLI restricts it to loopback and the known CGI path. The service verifies original loopback context, secret, activation and time window before sending the burst; repeated execution in the same target minute is prevented atomically. Normal browser requests need a DSM administrator session verified through `authenticate.cgi`, and writes also need a session-bound CSRF token.

The internal TaskScheduler API is not a stable public compatibility promise. Actual DSM 7.1 methods, task timing objects, owner behavior and edit responses require NAS testing. A failed API registration remains visible and cannot appear as a successfully registered local-only schedule.

## Permissions and gateway

`conf/privilege` contains only `defaults.run-as: package`. Executables use `0755`, with no executable/tool overrides, setuid/setgid, capabilities or system-group changes. The builder rejects other configurations.

The package owns `target/run` (`0755`) and holds its socket there. Ordinary DSM CGI identities can connect locally; this does not replace authentication. State access requires a verified administrator session, writes require CSRF, scheduled wake requests require their secret and original loopback address. Shutdown requires the private package secret and is inaccessible through the CGI. A separate lock prevents concurrent services.

The gateway forwards the actual cookie/token, language, client/server address, port and scheme. Browser-supplied gateway metadata is replaced. Each authentication invocation receives a separate CGI environment; concurrent sessions share no process-wide cookie or token values. The CGI never opens private state files. Missing services return visible errors. CGI execution permissions and identity are DSM-specific and must be verified on the target NAS; neither root privileges nor signed-only Synology resources should be added to work around failures.

## Logs and notifications

Every wake action is durably recorded before UDP transmission and updated under the same ID afterward. Log Center delivery first uses `synologset1`, falling back to a configured loopback BSD/TCP receiver. Pending records are retained, alongside the newest 300 delivered entries. At 3000 pending records, additional wake actions are blocked before sending. An interrupted intent record alone proves neither packet delivery nor device startup. Oversized state is rejected before replacing readable committed data.

The package contains no `conf/resource` and requires no `sysnotify` worker. A previously attributed installation rejection belonged to another package and did not establish a SynoWake `sysnotify` failure.

Notifications invoke `synodsmnotify -c SYNO.SDS.SynoWake.Application -p plain @administrators SynoWake:notification:title <message-key> <device-name>`. The German and English catalogs contain title, generic message, `wake_sent` and `wake_failed`. DSM resolves the template for each recipient. Only the unchanged device name is supplied to `{0}` as a separate process argument. Error details stay in the application log. All four keys are preloaded so notifications work with the app closed. No shell or JSON mail string is used. Notification failures are diagnosed and logged; disabled schedules send neither packets nor notifications.

## References

- [INFO fields](https://help.synology.com/developer-guide/synology_package/INFO_optional_fields.html), [privileges](https://help.synology.com/developer-guide/privilege/privilege_config.html) and [FHS](https://help.synology.com/developer-guide/integrate_dsm/fhs.html).
- [Application authentication](https://help.synology.com/developer-guide/integrate_dsm/web_authentication.html).
- [Desktop notifications](https://help.synology.com/developer-guide/synology_package/show_massage.html), [application I18N](https://help.synology.com/developer-guide/integrate_dsm/i18n.html) and [application config](https://help.synology.com/developer-guide/integrate_dsm/config.html).
- [Log receiving](https://kb.synology.com/index.php/en-us/DSM/help/LogCenter/logcenter_server?version=7).
- Developer-published examples: [AutoPilot config](https://github.com/toafez/AutoPilot/blob/main/ui/config), [window integration](https://github.com/toafez/AutoPilot/blob/main/ui/AutoPilot.js), [N4S4 Task Scheduler](https://github.com/N4S4/synology-api/blob/master/synology_api/task_scheduler.py) and [API transport](https://github.com/N4S4/synology-api/blob/master/synology_api/auth.py).

## Device web interfaces

Optional `Device.webPorts` is a string; absent/empty means `5000, 80, 8080`, preserving state format version 1. Saving validates at most 16 decimal ports from 1 to 65535 and removes duplicates in input order. `POST device-pages` uses existing DSM authentication, CSRF and origin guards. It accepts only a saved device ID and reads IP/ports from that record. The address is revalidated against private or directly attached IPv4 NAS networks.

Only popup opening or explicit rechecking sends parallel HEAD requests, with a shared 8-second deadline, at most 3 seconds for response headers and 4 seconds per HTTP/HTTPS request, and bounded response headers, without proxy, cookies, session tokens or redirect following. HTTP and HTTPS are detected; self-signed TLS is accepted only for this unauthenticated detection. A refused TCP connection is not a web-interface match. Links require HTTP(S), the device IP and a configured port, without credentials, paths or query strings. `target="_blank"`, `rel="noopener noreferrer"` and `referrerpolicy="no-referrer"` isolate the browser page from the DSM opener. Late results from closed popups are ignored. Browser LAN access cannot be established reliably, so links remain available and require LAN/VPN access.
