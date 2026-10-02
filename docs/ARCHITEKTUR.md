# DSM-Anbindung

## Dateien und Prozesse

```text
SynoWake.spk
  INFO                         Paketkennung, apollolake, DSM-Mindestversion
  conf/privilege               Paketkonto; CGI-Datei mit Paketidentität
  conf/resource                DSM-Benachrichtigungstexte
  scripts/                     init/start/stop/status
  package.tgz
    bin/synowake               CLI für DSM-Aufgabenplaner und Paketlebenszyklus
    ui/api.cgi                 dieselbe statische Go-Binary als CGI
    ui/index.html              Browseroberfläche
    ui/app.js
    ui/scheduler.js            DSM-TaskScheduler-Client im Browser
    ui/style.css
    ui/config                  DSM-Anwendungsregistrierung
    ui/SynoWake.js              natives DSM-Fenster mit eingebetteter Oberfläche
    ui/images/                 durch Code gezeichnete PNG-Symbole
    ui/texts/                  deutsche und englische Benachrichtigungstexte
```

Es läuft kein zusätzlicher Webserver und kein eigener Cron-Daemon. DSM stellt die Dateien über `dsmuidir="ui"` unter `/webman/3rdparty/SynoWake/` bereit und startet den CGI-Endpunkt pro Anfrage. `start` und `stop` verwalten den Aktivzustand; `status` liefert 0 für aktiv und 3 für gestoppt.

Die private Datenablage liegt im DSM-Paketverzeichnis `var`, außerhalb des UI-Verzeichnisses. Speichern verwendet eine Dateisperre und atomaren Austausch, damit parallel laufende CGI-Anfragen denselben Datenbestand bearbeiten können. Der Zustand für angemeldete Administratoren enthält den Aufgabenbefehl mit dem jeweiligen Zeitplan-Schlüssel, damit die Anwendung ihre DSM-Aufgabe eindeutig prüfen und eine unterbrochene Registrierung wiederaufnehmen kann. Der Schlüssel erscheint auch im DSM-Aufgabenskript. Es gibt kein zusätzliches Geheimnisfeld im Oberflächenzustand; die Anwendung zeigt Schlüssel nicht im DOM an und schreibt sie nicht in das Ausführungsprotokoll.

## Aufgabenplaner

Die administrativ angemeldete Browseroberfläche verwendet `SYNO.Core.TaskScheduler` über den WebAPI-Endpunkt der vorhandenen DSM-Sitzung. DSM übernimmt Registrierung, Aktivierung und Zeitsteuerung. Das Paket installiert weder statische Aufgaben über `task_config.xml` noch direkten Inhalt in `/etc/crontab`.

Geplante Aufgaben starten das CLI mit Zeitplan-ID, Geheimnis und lokalem Callback. Das CLI beschränkt den Callback auf Loopback und die bekannte CGI-Adresse. Der CGI prüft Geheimnis, Aktivzustand und Zeitplan, bevor es ein Magic Packet sendet. Normale CGI-Anfragen benötigen eine gültige DSM-Administratorsitzung. Die DSM-Anmeldung wird über den dokumentierten `authenticate.cgi` geprüft; schreibende Browseranfragen benötigen zusätzlich den Anwendungs-CSRF-Schutz.

Die WebAPI-Version und Felder des Aufgabenplaners sind nicht Teil einer stabilen öffentlichen Schnittstellenzusage. Zu prüfende Punkte sind die auf DSM 7.1 verfügbaren Methoden, das Zeitplanobjekt, Tasks des angemeldeten Benutzers und Antworten beim Editieren. Ein API-Fehler muss in der Oberfläche sichtbar bleiben. Eine nur lokal gespeicherte Konfiguration darf nicht als registrierter DSM-Zeitplan erscheinen.

## Berechtigungen

`conf/privilege` verwendet `run-as: package`. Die CGI-Datei wird dieser Identität zugeordnet. Zusätzlich setzt die dokumentierte `tool`-Konfiguration `ui/api.cgi` auf Besitzer/Gruppe `package` und Modus `4755`. Das Setuid-Bit gibt dem ELF-Programm die effektive Identität des **SynoWake-Paketkontos**, damit die private Dateiablage unabhängig von der DSM-CGI-Dispatcheridentität erreichbar bleibt. Es gibt keine Root-Lebenszyklusskripte und keine Änderung von Systemgruppen. Das reguläre CLI `bin/synowake` bleibt `0755`.

Bei einer Ausführung mit abweichender effektiver UID sind reine CLI-Aufrufe gesperrt. CGI-Datenpfad und Suchpfad für Systemprogramme sind festgelegt; ein fremder Umgebungswert darf den CGI-Datenpfad nicht überschreiben. HTTP-Anfragen benötigen weiterhin DSM-Authentifizierung oder den lokalen Zeitplan-Schlüssel. UDP-Broadcast benötigt keinen privilegierten Port. Die DSM-Annahme des Paketkonto-Setuid-Modus sowie die Verfügbarkeit von `ping`, Authentifizierungsprüfung, `synologset1` und `synodsmnotify` werden auf der NAS geprüft.

Die Ausführbarkeit von `authenticate.cgi` und die CGI-Benutzeridentität sind DSM-spezifisch. Falls DSM die Authentifizierungsprüfung unter dem Paketkonto nicht erlaubt, muss der Zugriff gesperrt bleiben. Zum Beheben die DSM-Anbindung untersuchen; nicht die Prüfung umgehen oder das Programm als Root betreiben.

## Log- und Benachrichtigungsintegration

Der eigene Ausführungsverlauf ist die erste Aufzeichnung. Ein DSM-Systemprotokolleintrag wird über `synologset1 sys ...` versucht. Die verwendete System-Log-ID ist eine DSM-interne Konvention und muss ebenso wie die Rechte des Paketkontos geprüft werden. Als Ersatz kann ein Administrator einen lokalen TCP-Empfangsport des Protokoll-Centers einstellen. SynoWake sendet dann BSD-Syslog-Nachrichten an `127.0.0.1` und den konfigurierten Port. Externe Ziele sind nicht vorgesehen.

Scheitert die Übertragung, bleibt der Eintrag mit einer Versandmarkierung im lokalen Verlauf. Administratoren können den Versand nach Beheben des Empfängers wiederholen. Ein erfolgreicher TCP-Schreibvorgang bestätigt die Übertragung an den Empfänger, liefert aber keine Anwendungsbestätigung der Log-Center-Datenbank. Sichtprüfung und Ende-zu-Ende-Test bleiben erforderlich.

Alle ausstehenden Einträge bleiben erhalten; bereits übertragene Einträge werden auf die neuesten 300 begrenzt. Bei 3000 ausstehenden Einträgen blockiert SynoWake weitere Wake-Aktionen vor dem UDP-Versand. Die lokale Absicht wird vor dem Versand dauerhaft geschrieben und unter derselben Eintrags-ID mit dem Ergebnis ergänzt. Ein Prozessabbruch kann einen begonnenen Eintrag hinterlassen; dieser belegt allein keinen erfolgreichen Versand oder Gerätestart. Der Dateischreiber begrenzt den Datenbestand auf ein lesbares Format und meldet einen Fehler, bevor eine zu große Datendatei ersetzt würde.

Für Desktop-Meldungen registriert die Ressource `sysnotify` die Kategorie `SynoWake` und bindet sie an `SYNO.SDS.SynoWake.Application`. Das Backend übergibt den Text als JSON-Variable `%MESSAGE%` an `synodsmnotify`. Die Ressource wird bei Paketstart erworben und beim Stoppen freigegeben; gestoppte Zeitpläne sollen deshalb weder Wake-Aktionen noch Meldungen auslösen.

## Quellen

- [Synology INFO-Felder](https://help.synology.com/developer-guide/synology_package/INFO_optional_fields.html): `dsmuidir`, `dsmappname`, Prüfsumme und Paketsteuerung.
- [Synology Privilege Config](https://help.synology.com/developer-guide/privilege/privilege_config.html): Paketidentität und ausführbare Dateien.
- [Synology FHS](https://help.synology.com/developer-guide/integrate_dsm/fhs.html): separate `target`- und `var`-Verzeichnisse.
- [Synology Application Authentication](https://help.synology.com/developer-guide/integrate_dsm/web_authentication.html): Prüfung der DSM-Sitzung aus einer Paket-CGI.
- [Synology System Notification](https://help.synology.com/developer-guide/resource_acquisition/sysnotify.html): Ressource, Kategorie und Aufruf mit benutzerdefinierten Variablen.
- [Synology Log Receiving](https://kb.synology.com/index.php/en-us/DSM/help/LogCenter/logcenter_server?version=7): Protokoll-Center-Empfänger mit BSD-Format, TCP und frei wählbarem Port.
- [AutoPilot config](https://github.com/toafez/AutoPilot/blob/main/ui/config) und [AutoPilot.js](https://github.com/toafez/AutoPilot/blob/main/ui/AutoPilot.js): eigener Quelltext als Beispiel des DSM-Anwendungsfensters.
- [N4S4 Task Scheduler](https://github.com/N4S4/synology-api/blob/master/synology_api/task_scheduler.py) und [WebAPI-Transport](https://github.com/N4S4/synology-api/blob/master/synology_api/auth.py): primärer Projektquelltext einer inoffiziellen API-Implementierung, als Referenz für Aufgabenfelder und Request-Format.
