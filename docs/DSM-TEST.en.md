# Acceptance on DS918+ / DSM 7.1

[Deutsch](DSM-TEST.md) · English

This checklist describes testing on the target NAS. A local build or browser demo does not prove successful DSM installation. Record the DSM version/build and NAS time zone. One test computer with a known MAC and working Wake-on-LAN is sufficient for the initial run.

| Test | Expected result |
| --- | --- |
| Install the SPK manually | Package Center accepts the architecture and minimum version; no root-privilege or restricted-resource rejection. |
| Inspect executable permissions | `ui/api.cgi` and `bin/synowake` use `0755`, without setuid/setgid or additional execution privileges. |
| Inspect the package service | Runs as the SynoWake package account; `target/run/backend.sock` exists; `bin/synowake status` returns 0. |
| Start and open the package | A DSM window displays overview tiles, devices and automation. |
| Switch DSM between German and English | Tabs, dialogs, status, discovery notices, errors and logs follow DSM language. Existing names remain unchanged. |
| Open the interface directly with German/English browser preferences | Without DSM language information, browser language is used; other languages fall back to English. |
| Access without DSM login | `api.cgi` rejects data access and wake actions. |
| Access as an ordinary DSM user | Backend denies administrator operations. |
| Save a device with name, IP and MAC | A row appears without a favorite check; its name can be edited. |
| Enable and disable a favorite | Only favorites appear as tiles. The choice survives reload/restart; every device remains available in the list and automation. |
| Wake favorites using Select all | Only visible tiles are selected. Removing a favorite removes it from quick selection. |
| Enter an invalid IP or MAC | A visible validation error appears; no packet is sent. |
| Open discovery with an empty device inventory | A connected NAS network is suggested, with no fixed `192.168.1.0/24`. |
| Test multiple active interfaces | Interface, NAS IP and actual CIDR are selectable. Inactive, loopback and `169.254.*` interfaces are absent. |
| Search a connected `/24`, including nonprivate ranges | Reachable neighbors with MAC addresses appear and can be saved with their interface broadcast address. |
| Import search results | All checks start empty. Only selected new results are imported, initially without favorite status. |
| Discover saved devices again | Results remain visible with Already saved and their saved names; import/name controls are disabled. MAC matching also works with changed IP addresses. |
| Inspect discovered names | Label is Name. `ACER-Frank.fritz.box` is suggested as `ACER-Frank` and remains editable. |
| Reopen after a successful custom search | The saved range is offered after DSM reload/package restart if still connected. |
| Test connected `/25` through `/30` networks | Actual masks are retained; ranges exceeding the connected network are rejected. |
| Wake one or several devices | Three identical Magic Packets reach each target, spaced about 20 ms apart. One execution record per device; no status-dependent retransmission. Status changes from Waking up to Online when a probe proves reachability. |
| ICMP is unavailable to the package account | Discovery uses TCP, populates neighbor tables and explains the fallback. Connection/refusal means Online; no response means Unknown. |
| ICMP works but target does not respond | Offline means no ICMP response, without proving that the target is powered off. |
| Save a schedule for the next minute | Exactly one corresponding SynoWake task exists in DSM Task Scheduler. |
| Save with an empty or whitespace-only name | Generated name contains device, days and time: Daily, Mon–Fri or ordered individual days, localized to the interface. DSM uses the same name with the SynoWake prefix. |
| Save with a custom name | Entered name is preserved. |
| Open via DSM; account can create native tasks | Creation uses DSM's session transport and current protection without erroneous error 105. |
| Lose the native request's response | No raw fallback or duplicate creation. Preparation remains recoverable; a late response does not register it afterward. |
| Catalog advertises TaskScheduler version 3 | get/create/set prefer 3 and try compatible alternatives only after 104. list uses 3, delete 2; successful versions are cached per method. |
| DSM rejects create version 4 with 104 | A supported alternative is used; exactly one task is created. Complete rejection lists checked versions. |
| Interrupt task creation | No speculative retry using another version; preparation remains for reconciliation. |
| Execute a schedule | Target starts; the application log records time and device. |
| Edit time/weekdays | Existing DSM task changes without duplication. |
| Disable/enable a schedule | DSM task and interface state agree. |
| Enable notifications | A desktop success/failure message appears in the recipient's language. Disabled notifications remain absent. |
| Close the application before execution | Notifications still work through `preloadTexts`. Check German/English recipients and device names containing umlauts, braces, percent signs or angle brackets. |
| Open Log Center | The wake action is identifiable as SynoWake. Do not ignore an integration diagnostic. |
| Configure the TCP fallback | BSD/TCP receiver and saved port agree; loopback delivery appears in Log Center. |
| Temporarily disable reception | New actions remain marked undelivered locally, with a visible diagnostic. |
| Restore reception and retry | Queued entries are delivered; this never sends another wake packet. |
| Test logging backlog limits | Pending records survive pruning; 3000 pending entries block new wake actions before transmission. |
| Stop the package | Existing schedules send no packet. |
| Restart the package | Schedules can run again. |
| Reboot the NAS | Package and registered tasks work after startup. |
| Upgrade the package | Devices, favorites, schedules and logs remain; tasks are not duplicated. |
| Remove all SynoWake schedules | Corresponding DSM tasks disappear. |
| Uninstall afterward | No SynoWake executable path or associated task remains registered for execution. |

## Diagnosing a failure

Keep Package Center and widgets open while starting/stopping SynoWake and opening/closing its window. Text, row heights, icons and forms outside SynoWake must remain unchanged. Fully reload DSM after upgrading; for the described Firefox setup, use Shift+click on Refresh. Upgrading from before 0.1.5 leaves devices without an old favorite field unchecked; mark desired tiles once.

Record the visible error, DSM version, time and action. Inspect the application diagnostics; installation/startup issues also require `/var/log/packages/SynoWake.log` and DSM package logs. Private `/var/packages/SynoWake/var` data contains schedule secrets and must not be included in unredacted support reports.

For Task Scheduler errors, record code and method and inspect the task directly in DSM. A failed registration must remain an error and must not be accepted as a working schedule.

If the window is empty, try the direct interface after DSM login. If it works, inspect window registration. If `api.cgi` fails too, inspect service status, `var/service.log`, CGI execution and authentication. The gateway must reach the socket without access to private state files.

If actions appear locally but not in Log Center, inspect `synologset1` diagnostics. Choose a Log Center archive target and create a BSD/TCP receiver, with no SSL for this loopback sender. Save the same port in SynoWake, execute an action and verify its appearance. Retry pending entries after fixing reception. Successful TCP delivery alone does not prove Log Center database storage; visual verification is required.

## Record results

```text
NAS model: DS918+
DSM version/build:
NAS time zone:
SynoWake version:
SPK SHA256:
Installation / CGI / window:
German / English / unchanged names:
Devices / discovery / wake / status:
Packet count / spacing / execution record:
Schedule creation / editing / disabling / deletion:
Stop / start / reboot / upgrade:
Log Center:
DSM notifications:
Remaining tasks after removal:
Open defects:
```
