# DSM-Anbindung

## Dateien und Prozesse

```text
SynoWake.spk
  INFO                         Paketkennung, apollolake, DSM-Mindestversion
  conf/privilege               ausschließlich defaults.run-as: package
  scripts/                     init/start/stop/status
  package.tgz
    bin/synowake               Paketdienst und CLI für Aufgabenplaner/Lebenszyklus
    ui/api.cgi                 dieselbe Binary als unprivilegierte Weiterleitung
    run/backend.sock           zur Laufzeit erzeugter lokaler Unix-Socket
    ui/index.html              Browseroberfläche
    ui/app.js
    ui/scheduler.js            DSM-TaskScheduler-Client im Browser
    ui/style.css
    ui/config                  DSM-Anwendungsregistrierung
    ui/SynoWake.js              natives DSM-Fenster mit eingebetteter Oberfläche
    ui/images/                 durch Code gezeichnete PNG-Symbole
    ui/texts/                  deutsche und englische Benachrichtigungstexte
```

DSM stellt die Dateien über `dsmuidir="ui"` unter `/webman/3rdparty/SynoWake/` bereit und startet den CGI-Endpunkt pro Anfrage. Ein dauerhafter Paketdienst nimmt dessen Weiterleitungen über einen Unix-Socket entgegen. Er öffnet keinen TCP-Empfangsport. Der CGI-Endpunkt greift nicht auf Paketdaten zu. Das Startskript startet den Dienst unter dem Paketkonto und wartet auf eine erfolgreiche Zustandsprüfung. `stop` setzt den Aktivzustand zurück und beendet den Dienst über eine mit einem privaten Schlüssel geschützte Steueranfrage. `status` liefert 0 für aktiv und erreichbar, andernfalls 3. DSM bleibt für die Zeitplanung zuständig.

Die private Datenablage liegt im DSM-Paketverzeichnis `var`, außerhalb des UI-Verzeichnisses. Speichern verwendet eine Dateisperre und atomaren Austausch, damit parallele Dienstanfragen und Lebenszyklusbefehle denselben Datenbestand bearbeiten können. Der Zustand für angemeldete Administratoren enthält den Aufgabenbefehl mit dem jeweiligen Zeitplan-Schlüssel, damit die Anwendung ihre DSM-Aufgabe eindeutig prüfen und eine unterbrochene Registrierung wiederaufnehmen kann. Der Schlüssel erscheint auch im DSM-Aufgabenskript. Es gibt kein zusätzliches Geheimnisfeld im Oberflächenzustand; die Anwendung zeigt Schlüssel nicht im DOM an und schreibt sie nicht in das Ausführungsprotokoll.

## Aufgabenplaner

Die administrativ angemeldete Browseroberfläche verwendet `SYNO.Core.TaskScheduler` über den WebAPI-Endpunkt der vorhandenen DSM-Sitzung. DSM übernimmt Registrierung, Aktivierung und Zeitsteuerung. Das Paket installiert weder statische Aufgaben über `task_config.xml` noch direkten Inhalt in `/etc/crontab`.

Geplante Aufgaben starten das CLI mit Zeitplan-ID, Geheimnis und lokalem Callback. Das CLI beschränkt den Callback auf Loopback und die bekannte CGI-Adresse. Die CGI-Weiterleitung übermittelt den ursprünglichen Anfragekontext an den Dienst. Dieser prüft Loopback-Adresse, Geheimnis, Aktivzustand und Zeitplan, bevor er ein Magic Packet sendet. Normale Browseranfragen benötigen eine gültige DSM-Administratorsitzung. Die DSM-Anmeldung wird über den dokumentierten `authenticate.cgi` geprüft; schreibende Browseranfragen benötigen zusätzlich den Anwendungs-CSRF-Schutz.

Die WebAPI-Version und Felder des Aufgabenplaners sind nicht Teil einer stabilen öffentlichen Schnittstellenzusage. Zu prüfende Punkte sind die auf DSM 7.1 verfügbaren Methoden, das Zeitplanobjekt, Tasks des angemeldeten Benutzers und Antworten beim Editieren. Ein API-Fehler muss in der Oberfläche sichtbar bleiben. Eine nur lokal gespeicherte Konfiguration darf nicht als registrierter DSM-Zeitplan erscheinen.

## Berechtigungen

`conf/privilege` enthält nur `defaults.run-as: package`. Dadurch startet DSM die Lebenszyklusskripte und den Dienst unter dem Paketkonto. Die Binärdateien haben Modus `0755`. Es gibt keine `executable`- oder `tool`-Einträge, keine Setuid-/Setgid-Bits, keine Datei-Capabilities und keine Änderungen von Systemgruppen. Der Builder weist abweichende Privilegienkonfigurationen zurück.

Der Dienst hält den Socket unter `target/run/backend.sock`; das Verzeichnis gehört dem Paketkonto und hat Modus `0755`. Der Socket erlaubt der normalen DSM-CGI-Identität eine lokale Verbindung. Diese Verbindung ersetzt keine Anmeldung: Datenzugriffe benötigen weiterhin eine geprüfte DSM-Administratorsitzung und schreibende Anfragen einen sitzungsgebundenen CSRF-Schlüssel. Geplante Wake-Aufrufe benötigen ihren individuellen Schlüssel und eine ursprüngliche Loopback-Adresse. Steueraufrufe zum Stoppen benötigen das private Paketgeheimnis und sind über die CGI-Weiterleitung nicht erreichbar. Eine zusätzliche Dateisperre verhindert mehrere gleichzeitig laufende Dienste.

Die CGI-Weiterleitung übernimmt Cookie, SynoToken, Client-/Serveradresse, Port und Protokoll aus der tatsächlichen Anfrage. Vom Browser gelieferte Weiterleitungsmetadaten werden ersetzt. Der Dienst erzeugt für jeden Aufruf von `authenticate.cgi` eine eigene Umgebung; parallele Sitzungen teilen keine Cookie- oder Tokenvariablen. Der CGI-Prozess öffnet keine Datendateien. Bei einem fehlenden Dienst liefert er einen sichtbaren Fehler.

Die Ausführbarkeit von `authenticate.cgi` und die CGI-Benutzeridentität sind DSM-spezifisch. Falls DSM die Authentifizierungsprüfung unter dem Paketkonto nicht erlaubt, muss der Zugriff gesperrt bleiben. Zum Beheben die DSM-Anbindung untersuchen; nicht die Prüfung umgehen oder das Programm als Root betreiben.

## Log- und Benachrichtigungsintegration

Der eigene Ausführungsverlauf ist die erste Aufzeichnung. Ein DSM-Systemprotokolleintrag wird über `synologset1 sys ...` versucht. Die verwendete System-Log-ID ist eine DSM-interne Konvention und muss ebenso wie die Rechte des Paketkontos geprüft werden. Als Ersatz kann ein Administrator einen lokalen TCP-Empfangsport des Protokoll-Centers einstellen. SynoWake sendet dann BSD-Syslog-Nachrichten an `127.0.0.1` und den konfigurierten Port. Externe Ziele sind nicht vorgesehen.

Scheitert die Übertragung, bleibt der Eintrag mit einer Versandmarkierung im lokalen Verlauf. Administratoren können den Versand nach Beheben des Empfängers wiederholen. Ein erfolgreicher TCP-Schreibvorgang bestätigt die Übertragung an den Empfänger, liefert aber keine Anwendungsbestätigung der Log-Center-Datenbank. Sichtprüfung und Ende-zu-Ende-Test bleiben erforderlich.

Alle ausstehenden Einträge bleiben erhalten; bereits übertragene Einträge werden auf die neuesten 300 begrenzt. Bei 3000 ausstehenden Einträgen blockiert SynoWake weitere Wake-Aktionen vor dem UDP-Versand. Die lokale Absicht wird vor dem Versand dauerhaft geschrieben und unter derselben Eintrags-ID mit dem Ergebnis ergänzt. Ein Prozessabbruch kann einen begonnenen Eintrag hinterlassen; dieser belegt allein keinen erfolgreichen Versand oder Gerätestart. Der Dateischreiber begrenzt den Datenbestand auf ein lesbares Format und meldet einen Fehler, bevor eine zu große Datendatei ersetzt würde.

Das Paket enthält keine `conf/resource` und fordert keine Ressourcen-Worker an. DSM 7.1 hat die frühere `sysnotify`-Registrierung als Ressource ausschließlich für Synology-Pakete abgewiesen; der Builder verhindert deren erneute Aufnahme.

Für Desktop-Meldungen verwendet der Dienst `synodsmnotify -c SYNO.SDS.SynoWake.Application -p plain @administrators SynoWake:notification:title SynoWake:notification:message <Text>`. Die Sprachdateien `ui/texts/{ger,enu}/strings` enthalten Titel und den Nachrichtenplatzhalter `{0}`. `texts` und `preloadTexts` sind im App-Config eingetragen, damit Meldungen auch bei geschlossenem App-Fenster möglich sind. Der Text wird als eigenes Prozessargument übergeben, ohne Shell oder JSON-Mailstring. Fehler erscheinen als Diagnose und lokaler Protokolleintrag. Gestoppte Zeitpläne senden weiterhin weder Magic Packet noch Benachrichtigung.

## Quellen

- [Synology INFO-Felder](https://help.synology.com/developer-guide/synology_package/INFO_optional_fields.html): `dsmuidir`, `dsmappname`, Prüfsumme und Paketsteuerung.
- [Synology Privilege Config](https://help.synology.com/developer-guide/privilege/privilege_config.html): Paketidentität und ausführbare Dateien.
- [Synology FHS](https://help.synology.com/developer-guide/integrate_dsm/fhs.html): separate `target`- und `var`-Verzeichnisse.
- [Synology Application Authentication](https://help.synology.com/developer-guide/integrate_dsm/web_authentication.html): Prüfung der DSM-Sitzung aus einer Paket-CGI.
- [Synology Desktop Notifications](https://help.synology.com/developer-guide/synology_package/show_massage.html), [Application I18N](https://help.synology.com/developer-guide/integrate_dsm/i18n.html) und [App-Config](https://help.synology.com/developer-guide/integrate_dsm/config.html): App-I18N-Schlüssel, Sprachdateien und `preloadTexts`.
- [Synology Log Receiving](https://kb.synology.com/index.php/en-us/DSM/help/LogCenter/logcenter_server?version=7): Protokoll-Center-Empfänger mit BSD-Format, TCP und frei wählbarem Port.
- [AutoPilot config](https://github.com/toafez/AutoPilot/blob/main/ui/config) und [AutoPilot.js](https://github.com/toafez/AutoPilot/blob/main/ui/AutoPilot.js): eigener Quelltext als Beispiel des DSM-Anwendungsfensters.
- [N4S4 Task Scheduler](https://github.com/N4S4/synology-api/blob/master/synology_api/task_scheduler.py) und [WebAPI-Transport](https://github.com/N4S4/synology-api/blob/master/synology_api/auth.py): primärer Projektquelltext einer inoffiziellen API-Implementierung, als Referenz für Aufgabenfelder und Request-Format.
