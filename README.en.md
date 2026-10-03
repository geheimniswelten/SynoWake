# SynoWake

[Deutsch](README.md) · English

Wake-on-LAN application for a Synology **DS918+ running DSM 7.1**. The internal package ID is `h5uSynoWake`; the DisplayName and visible DSM application name remain **SynoWake**.

The application opens in a DSM window with favorite device tiles, a searchable device list and an automation tab with execution logs. Schedules are created, edited and deleted through the existing DSM session using `SYNO.Core.TaskScheduler`. SynoWake does not modify `/etc/crontab` or `task_config.xml`.

**Version 1.0.0-0017:** The package version, runtime display and cache URLs use `1.0.0-0017`. The BETA flag is removed; build number `0017` is retained.

**Version 0.1.16-0017: more time for web interfaces.** The HTTP-header deadline increases from 800 ms to 3 seconds; each HTTP/HTTPS request may take up to 4 seconds. Configured ports are still checked in parallel, with a shared maximum of 8 seconds. This accommodates delayed DSM login pages instead of missing them after less than a second. A regression test deliberately delays an HTTP response by 1.2 seconds. The precise cause of the earlier intermittent NAS results remains unconfirmed.

**Version 0.1.15-0016: device web interfaces.** Device settings accept up to 16 web ports. An empty field uses `5000, 80, 8080`, including existing records. Click **Online ↗** on a tile or device row to open the popup. The NAS checks those ports for HTTP/HTTPS, including self-signed TLS for detection, and offers responding web interfaces as links. Probes do not follow redirects or forward cookies or DSM session data. Links open new browser tabs without access to the DSM opener or its referrer. Normal browser certificate checks still apply when opening a page. The popup supports checking again. Browser LAN access cannot be established reliably, so the popup remains available; device-IP links require LAN/VPN access. Regular online checks are independent of these web ports.

**Version 0.1.14-0015: schedule deletion on DSM 7.1 fixed.** The complete inventory uses `list` version 2, matching Synology's native interface. Only this read-only method may try version 3 after explicit method/version rejection (103/104). Owner, SynoWake prefix and exact wake command are still checked before deletion. Ambiguous deletion responses are reconciled using a complete inventory without another delete request. Creation, editing and enable/disable were confirmed on the target NAS; the corrected deletion flow was also confirmed on the NAS.

**Version 0.1.13-0014: short WoL intervals and complete German/English product texts.** Each wake action sends three identical Magic Packets with a 20 ms pause between packets. The burst remains one wake action with one execution record. Missing status responses do not trigger another transmission.

The interface, forms, status labels, validation, Task Scheduler messages, discovery notices, log messages and notification templates are available in German and English. The interface prefers `SYNO.SDS.Session.lang` from DSM, then the browser language; other languages fall back to English. A direct preview can use `?lang=de` or `?lang=en`. Requests forward the chosen language through `Accept-Language`. Saved device names, schedule names and DNS names remain unchanged. Only sample names and generated suggestions are translated. New logs store the template and arguments separately so the same action can be displayed in either language. Technical operating-system details remain verbatim. Lifecycle and Log Center messages use the package process's `SYNOPKG_DSM_LANGUAGE`, with English as the fallback.

Package metadata identifies `himitsu` as developer ([website](https://geheimniswelten.de)) and `geheimniswelten` as publisher, with the planned [GitHub project](https://github.com/geheimniswelten/SynoWake) and its Issues page as support contact. These addresses are prepared for the repository that has not yet been published. `report_url` is omitted. The metadata does not determine DSM user permissions. `thirdparty="yes"` remains present; [Synology documents this field](https://help.synology.com/developer-guide/synology_package/INFO_optional_fields.html) as unused since DSM 5.0. Official Package Center distribution requires a separate application and review.

Schedule names are optional. An empty or whitespace-only name becomes `device · days · time`, for example `Work PC · Mon–Fri · 08:00`. All days use `Daily`; individual days are ordered Monday through Sunday. Custom names remain unchanged. Only the device portion of an automatically generated name is shortened to meet the 80-character limit.

Task creation through the native DSM session was confirmed on the target NAS in version 0.1.9. SynoWake uses the parent document's `SYNO.API.Request`, which handles the current session protection including `X-SYNO-HASH`. Error 105 describes a rejected session request rather than assuming that the user is not an administrator. Method versions 2–4 are considered for get/create/set, preferring catalog-advertised versions; list prefers version 2, with version 3 as an alternative only after explicit 103/104 rejection; delete uses 2. Write methods try another version only after error 104. Ambiguous writes and native transport failures never trigger speculative retries or a raw fallback request.

Saved discovery results remain visible and are marked **Already saved**. Their saved names and import controls are protected. Matching uses the MAC address even if the IP changes. The inventory is refreshed before import, and the backend prevents creating another device with the same MAC. New results start unchecked and newly imported devices are not favorites. Only favorites appear as tiles; all devices remain available in the list and automation.

The interface loads private `ui/assets/synowake.css` inside its iframe. Selectors are scoped to `#synowake-app`; the builder rejects a global `ui/style.css`. Versioned HTML, CSS and module URLs prevent stale asset combinations. Fully reload DSM after upgrading; for the described Firefox setup, use **Shift+click on Refresh**. Browser tests check unchanged styles and dimensions outside the application; real DSM integration still requires NAS verification.

## Build and tests

The development machine requires **Python 3.10+** and **Go 1.24+**. The NAS requires no Go, Python, PHP, Web Station or Container Manager: the package contains a static Linux/amd64 binary and static interface assets.

```powershell
python scripts/build.py
```

Or package a previously built static binary:

```powershell
python scripts/build.py --binary build/synowake --output dist/h5uSynoWake.spk
```

The builder validates a static Linux/amd64 ELF, produces an unsigned SPK and SHA256 file, and adds DSM's MD5 checksum for `package.tgz`. Paths, ordering, timestamps, line endings and POSIX permissions are fixed, producing identical archives from identical inputs. The package targets `apollolake`, requires at least DSM `7.1-42661` and has no BETA flag.

Run `go test ./...`, `node tests/scheduler.cjs`, `node tests/i18n.cjs` and `python tests/check-i18n.py`. The backend tests cover packet count/spacing, one execution record, concurrent language requests and unchanged names. The shared catalog is `internal/synowake/translations.json`: Go embeds it and the builder exports `ui/translations.js`. Build checks enforce matching template parameters and versioned language-module URLs.

`node tests/workflow.cjs` uses Playwright to test German and English workflows, favorite management, device import, simulated DSM API/session responses, recovery and mobile layout. Set `PLAYWRIGHT_MODULE` to the installed library and `BROWSER_EXECUTABLE` to Chromium or Edge if needed. Screenshots and CSS results are written under `work/`. `node scripts/test-ui.mjs` serves the CSS regression page for manual Firefox verification; the legacy fixture is never included in the SPK.

## Installation and first use

**Package ID transition:** DSM treats `h5uSynoWake` as a separate package; it does not automatically upgrade an existing installation identified as `SynoWake`. State and DSM tasks are not transferred automatically. Secure any needed data first, remove old schedules through the old application, then uninstall the old package. Recreate devices and schedules in the new package. The package ID determines package identity and its directory, as described in [Synology's INFO documentation](https://help.synology.com/developer-guide/synology_package/INFO_necessary_fields.html).

1. Sign in to DSM using an administrator account.
2. Open **Package Center → Manual Install** and select the SPK. DSM displays a publisher warning for this private package; verify its source and contents before continuing.
3. Start the package and open **SynoWake** from the main menu or Package Center.
4. In **Devices**, enter a name, IPv4 LAN address and MAC. Private addresses and addresses on directly connected NAS networks are accepted. Names remain editable; adjust broadcast address and UDP port if necessary.
5. Enable Wake-on-LAN in the target's BIOS/UEFI, operating system and network driver. Test on the same wired LAN first.
6. Mark devices as **Favorite** or enable **Show as a tile in the overview** when editing. Wake them using tiles or selection. Favorites do not restrict the device list or automation.
7. In **Automation**, choose the device, time, weekdays and optional DSM notifications. Times follow the NAS time zone.

If the native window does not load, open `https://NAS:DSM-Port/webman/3rdparty/h5uSynoWake/index.html` after signing in to DSM, using your configured HTTPS port. This helps diagnose window registration. Open SynoWake through DSM for automation: a directly opened page lacks the parent DSM request layer, so direct WebAPI requests may be rejected by DSM session protection.

## Status and discovery

**Online** means the NAS received an ICMP response or, if ICMP is unavailable, a TCP connection or explicit connection refusal. **Offline** means no response with usable ICMP; a firewall may cause a running device to appear offline. If neither ICMP nor TCP provides evidence, status is **Unknown**. After sending the burst, status remains **Waking up** for up to 90 seconds while checking. Sending packets does not prove that the device started.

The TCP fallback probes ports 445, 80, 443, 22, 3389 and 5000 in parallel with an overall 800 ms deadline per device. It sends no credentials or application commands. Discovery explains the fallback; status tooltips identify TCP probes. The package requests no raw-socket privileges, root execution, capabilities or kernel changes. Fully filtered devices cannot reliably be identified as switched off.

Discovery uses active NAS interfaces and their actual netmasks, preferring the interface used to reach DSM. Multiple interfaces are selectable. Loopback, inactive interfaces and `169.254.*` are excluded. No static `192.168.1.0/24` or saved device address is used as a default. If detection fails, the field remains empty. An interface/backend version mismatch produces a visible notice.

After successful discovery the normalized range is saved. It is proposed again after browser or package restarts if still connected; otherwise a current interface range is used. Search accepts `/24` through `/30` entirely within a connected interface network, up to 254 hosts. Nonprivate but directly connected ranges such as `192.167.178.0/24` are accepted. Larger interface networks propose the `/24` containing the NAS address; smaller networks retain their masks.

The search probes hosts and reads neighbor tables. Import selected new IP/MAC results with their interface's broadcast address. Reverse DNS suggestions remove `.fritz.box`, `.local` and `.lan`, for example `ACER-Frank.fritz.box` → `ACER-Frank`. Saved and manually entered names are never automatically renamed. Sleeping devices, remote VLANs and hosts without usable neighbor entries can be missing; enter them manually. DHCP reservations help prevent saved IP addresses from changing.

## Schedules and package lifecycle

Automation appears as SynoWake tasks in DSM Task Scheduler. Creating, editing or deleting tasks requires a live DSM administrator session. SynoWake does not store passwords.

Tasks invoke the included CLI, which calls the fixed local DSM CGI endpoint. The CGI forwards requests to the package service over a Unix socket. Each schedule has its own secret. Private state is under `/var/packages/h5uSynoWake/var`; programs are under `/var/packages/h5uSynoWake/target`. `conf/privilege` contains only `defaults.run-as: package`, programs use `0755`, and no extra execution privileges, setuid, group changes or capabilities are requested.

Stopping the package disables execution and stops the service. Registered tasks remain but cannot wake devices while the package is stopped. Starting restores the service; upgrades within the `h5uSynoWake` package ID preserve package data. Startup failures are logged in `var/service.log`.

**Remove all SynoWake schedules in the application before uninstalling.** Lifecycle scripts have no administrator session and cannot independently delete DSM tasks. Check for remaining SynoWake-prefixed tasks in DSM Task Scheduler. Such tasks point to a removed executable after package uninstallation.

## Logs and notifications

Every wake action is stored in the local log and attempted through DSM `synologset1`. If the package account cannot use direct logging, configure a local TCP syslog receiver in Log Center and enter its port under Automation. Integration failures remain visible and undelivered entries are queued. Visibility in the actual DSM 7.1 Log Center must be verified on the NAS.

Pending records are retained along with the newest 300 delivered entries. At 3000 pending entries, further wake actions are blocked until delivery works and the queue is reduced. The intent is stored before packet transmission and then updated under the same record ID, preserving evidence of interrupted execution.

For the fallback, first choose an archive destination in full **Log Center → Archive Settings**. Under **Log Receiving**, create a **BSD (RFC 3164), TCP** receiver on a free port, commonly 514. Disable SSL for this local sender. Save the same port in SynoWake; it sends only to `127.0.0.1`. If applicable, allow local traffic through DSM firewall rules. Execute an action, verify its Log Center entry and retry queued entries. The additional receiver is initially disabled (`0`). See [Synology's log receiving guide](https://kb.synology.com/index.php/en-us/DSM/help/LogCenter/logcenter_server?version=7).

When enabled for a schedule, `synodsmnotify` sends a desktop notification to the administrators group. DSM resolves the success/failure template in each recipient's language. Only the unchanged device name is passed as an argument to `{0}`. Failures refer to the SynoWake log for technical details. The German/English templates and title are in `ui/texts/{ger,enu}/strings`; `preloadTexts` makes them available even when the application window is closed. No `sysnotify` resource registration is required. Notification failures are visible in the application log.

See [NAS acceptance tests](docs/DSM-TEST.en.md) and [architecture](docs/ARCHITECTURE.en.md).

## DSM references

Package integration follows the [Synology Package Developer Guide](https://help.synology.com/developer-guide/): [INFO](https://help.synology.com/developer-guide/synology_package/INFO.html), [privileges](https://help.synology.com/developer-guide/privilege/privilege_config.html), [authentication](https://help.synology.com/developer-guide/integrate_dsm/web_authentication.html), [notifications](https://help.synology.com/developer-guide/synology_package/show_massage.html) and [I18N](https://help.synology.com/developer-guide/integrate_dsm/i18n.html). The current guide targets DSM 7.2.2; SynoWake targets 7.1, so documentation does not replace testing on the target firmware.

[AutoPilot's window integration](https://github.com/toafez/AutoPilot/blob/main/ui/AutoPilot.js) provides a developer-published example. The [N4S4 Task Scheduler implementation](https://github.com/N4S4/synology-api/blob/master/synology_api/task_scheduler.py) and [API transport](https://github.com/N4S4/synology-api/blob/master/synology_api/auth.py) provide unofficial reference code. Task Scheduler remains an internal DSM API whose concrete behavior must be tested on the target system.
