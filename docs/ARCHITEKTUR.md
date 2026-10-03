# DSM-Anbindung

Deutsch · [English](ARCHITECTURE.en.md)

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
    ui/i18n.js                 Auswahl der Sprache und Textformatierung
    ui/translations.js         aus dem gemeinsamen Katalog erzeugte Übersetzungen
    ui/assets/synowake.css      nur für den iframe, Selektoren unter #synowake-app
    ui/config                  DSM-Anwendungsregistrierung
    ui/SynoWake.js              natives DSM-Fenster mit eingebetteter Oberfläche
    ui/images/                 durch Code gezeichnete PNG-Symbole
    ui/texts/                  deutsche und englische Benachrichtigungstexte
```

DSM stellt die Dateien über `dsmuidir="ui"` unter `/webman/3rdparty/SynoWake/` bereit und startet den CGI-Endpunkt pro Anfrage. Ein dauerhafter Paketdienst nimmt dessen Weiterleitungen über einen Unix-Socket entgegen. Er öffnet keinen TCP-Empfangsport. Der CGI-Endpunkt greift nicht auf Paketdaten zu. Das Startskript startet den Dienst unter dem Paketkonto und wartet auf eine erfolgreiche Zustandsprüfung. `stop` setzt den Aktivzustand zurück und beendet den Dienst über eine mit einem privaten Schlüssel geschützte Steueranfrage. `status` liefert 0 für aktiv und erreichbar, andernfalls 3. DSM bleibt für die Zeitplanung zuständig.

Die private Datenablage liegt im DSM-Paketverzeichnis `var`, außerhalb des UI-Verzeichnisses. Speichern verwendet eine Dateisperre und atomaren Austausch, damit parallele Dienstanfragen und Lebenszyklusbefehle denselben Datenbestand bearbeiten können. Der Zustand für angemeldete Administratoren enthält den Aufgabenbefehl mit dem jeweiligen Zeitplan-Schlüssel, damit die Anwendung ihre DSM-Aufgabe eindeutig prüfen und eine unterbrochene Registrierung wiederaufnehmen kann. Der Schlüssel erscheint auch im DSM-Aufgabenskript. Es gibt kein zusätzliches Geheimnisfeld im Oberflächenzustand; die Anwendung zeigt Schlüssel nicht im DOM an und schreibt sie nicht in das Ausführungsprotokoll.

## Sprachen und WoL-Versand

Der gemeinsame Katalog `internal/synowake/translations.json` enthält deutsche Quelltexte und englische Übersetzungen. Go bettet ihn ein; `scripts/build.py` erzeugt daraus `ui/translations.js`. Textparameter müssen in beiden Vorlagen übereinstimmen. Die Oberfläche bevorzugt die DSM-Sprache, verwendet sonst die Browsersprache und für weitere Sprachen Englisch. `Accept-Language` wird durch die CGI an den Dienst weitergereicht; nur Meldungsfelder der Antwortkopie werden lokalisiert. Namen und gespeicherter Zustand bleiben unverändert. Neue Logeinträge enthalten zusätzlich eine Vorlage und separate Argumente. Die Formatierung setzt Parameter einmal ein, sodass geschweifte Klammern oder HTML-Zeichen in einem Gerätenamen keinen zweiten Formatierungsdurchlauf auslösen.

Der Linux-Sender schickt drei identische UDP-Magic-Packets mit zwei Pausen von jeweils 20 ms. Alle drei gehören zu derselben Weckaktion und aktualisieren denselben dauerhaften Ausführungseintrag. Bei einem Sendefehler bricht der Versand ab; Statusabfragen lösen keinen weiteren Versand aus. Separate spätere Statusmeldungen bleiben wie bisher möglich.

## Lokale Netzwerke

Der Zustand enthält pro Gerät ein boolesches `favorite`. Ohne dieses Feld gilt `false`; das bisherige Dateiformat bleibt lesbar. Der geschützte POST-Endpunkt `device-favorite` ändert atomar ausschließlich dieses Feld und erhält Adressen, Namen, Wake-Zustand und Zeitpläne. Übersicht, Statuszähler und Schnellauswahl verwenden ausschließlich Favoriten; Geräteliste und Automatiken behalten den gesamten Bestand.

Der Backend-Zustand enthält aktive IPv4-Schnittstellen mit NAS-Adresse, CIDR, begrenztem Suchvorschlag und Broadcast-Adresse. Loopback und IPv4-Link-Local werden ausgefiltert. Der ursprüngliche Serverkontext der DSM-CGI priorisiert die Schnittstelle, über die DSM erreicht wird; die Hostadresse dient als Ersatz. Der Suchdialog lädt diese Liste beim Öffnen neu und bietet die Schnittstellen zur Auswahl an.

Nach einer erfolgreichen Suche wird deren normalisierter Bereich als `discoveryCidr` gespeichert. Die Zustandsabfrage validiert ihn erneut gegen die aktuellen Schnittstellennetze, bevor sie ihn als Vorgabe liefert; ein nicht mehr angeschlossener Bereich wird durch den aktuellen Schnittstellenvorschlag ersetzt. DNS-Namen erhalten für die Übernahme keine bekannten lokalen Endungen `.fritz.box`, `.local` oder `.lan`. HTML, JavaScript-Module und CSS verwenden versionierte URLs; der Builder überprüft ihre Übereinstimmung mit INFO.

Suchtreffer werden anhand ihrer normalisierten MAC mit dem Gerätebestand verglichen. Bereits vorhandene Geräte zeigen ihren gespeicherten Namen und eine Markierung; ihre Übernahme ist gesperrt. Eine erneute Zustandsabfrage vor dem Import aktualisiert diese Markierungen, ohne bearbeitete neue Namen oder deren Auswahl zu verwerfen. Die atomare `device-save`-Transaktion verweigert zusätzlich neue Geräte-IDs für eine bereits vorhandene MAC. Vorhandene Einträge bleiben über ihre ID bearbeitbar.

Ein Suchnetz muss vollständig in einem angeschlossenen Schnittstellennetz liegen. Die Adressklasse allein entscheidet nicht über die Zulässigkeit: Ein lokal verwendetes `192.167.178.0/24` ist zulässig, ein nicht angeschlossenes `192.168.1.0/24` bleibt unzulässig. Größere Netze werden für den Suchvorschlag auf den `/24`-Bereich mit der NAS-Adresse begrenzt; `/25` bis `/30` bleiben unverändert. Die Validierung verwendet Netzanfang und Netzende, damit ein Suchbereich keine enger konfigurierte Netzmaske überschreitet.

Die Gerätevalidierung akzeptiert private IPv4-Adressen und zusätzliche direkt angeschlossene IPv4-Hostadressen. Für zusätzliche Broadcast-Ziele muss die Adresse mit dem Broadcast einer aktiven NAS-Schnittstelle übereinstimmen. Die Gerätesuche liefert diesen Broadcast zusammen mit IP und MAC, und die Oberfläche übernimmt ihn beim Speichern. Der allgemein gültige lokale Broadcast `255.255.255.255` bleibt verfügbar.

## Begrenzung der Oberflächen-Styles

Das für DSM registrierte UI-Verzeichnis enthält keine `style.css`. Diese reservierte Desktop-CSS-Datei darf keine globalen Resets einer eingebetteten HTML-Anwendung enthalten. `index.html` lädt stattdessen ausdrücklich `assets/synowake.css`. Sämtliche Selektoren, einschließlich Media-Queries und Dialogen, beginnen mit `#synowake-app`; dieser Bezeichner sitzt auf dem iframe-Body und existiert nicht im DSM-Dokument. Auch die Animation hat einen paketbezogenen Namen. Ein Browser-Test lädt die private CSS absichtlich im äußeren Dokument und vergleicht Stile und Abmessungen von Tabellen, Widgets und Formularelementen davor und danach. Die Build-Prüfung verhindert die erneute Auslieferung einer globalen `ui/style.css`.

## Aufgabenplaner

Die administrativ angemeldete Browseroberfläche verwendet `SYNO.Core.TaskScheduler` über `window.parent.SYNO.API.Request` der vorhandenen DSM-Sitzung. DSM übernimmt Registrierung, Aktivierung und Zeitsteuerung. Typisierte Parameter werden kopiert übergeben; DSM übernimmt Serialisierung, Sitzungstoken und den dynamischen Sitzungsschutz `X-SYNO-HASH`. Passwörter oder Hashes werden nicht ausgelesen, gespeichert oder nachgebildet. Die auf der Ziel-NAS ausgelieferten Dateien `TaskSchedulerUtils.js`, `sds.js`, `synowebapi.min.js` und `synocredential.min.js` wurden für diese Anbindung lesend geprüft. Der native Aufgabenplaner verwendet dort Version 3 für create/get/set. Das Paket installiert weder statische Aufgaben über `task_config.xml` noch direkten Inhalt in `/etc/crontab`.

API-Informationen werden zuerst über `entry.cgi` abgefragt, bei dort fehlendem Pfad oder HTTP 404 über `query.cgi`. get/create/set berücksichtigen die kompatiblen Versionen 2 bis 4, zunächst in absteigender Reihenfolge innerhalb der Katalogangabe, danach bekannte neuere Versionen bei veraltetem Katalog. Die gemeldete Mindestversion wird eingehalten. Eine erfolgreiche Version wird getrennt pro Methode für die laufende Oberfläche gemerkt. list verwendet Version 3, delete Version 2. Ausschließlich Fehler 104 (Versionsablehnung vor Ausführung) erlaubt einen weiteren Versuch mit einer anderen Version. Berechtigungsfehler 105, andere Fehlercodes, HTTP-Fehler und unklare Antworten lösen keine Wiederholung aus. Unterbrochene oder unklare Antworten behalten die vorbereitete Zuordnung für den bestehenden Abgleich. Der native Aufruf hat eine zusätzliche 22-Sekunden-Frist; spätere Antworten ändern den bereits abgeschlossenen Ausgang nicht. Nach einem nativen Fehler erfolgt kein direkter Ersatzaufruf. Ist die native Funktion vor dem Aufruf nicht verfügbar, etwa bei separat geöffneter Oberfläche, wird der bisherige direkte WebAPI-Aufruf mit Sitzungstoken als Header und Parameter verwendet; dieser unterstützt den zusätzlichen DSM-Sitzungsschutz nicht vollständig.

Die Erreichbarkeit wird zunächst per ICMP geprüft. Lokale Ausführungsfehler, auch Berechtigungsfehler mit Exitcode 1, lösen eine TCP-Prüfung aus. Ein erfolgreicher Connect oder ECONNREFUSED zeigt eine IP-Antwort; Timeouts ergeben „Unbekannt“. Die TCP-Prüfung benötigt keine erhöhten Rechte und aktiviert nebenbei die normale ARP-Auflösung des Kernels. Nachbartabellen bleiben die Quelle für MAC-Adressen. Weder Kernelparameter noch Systemdateirechte werden verändert.

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

Das Paket enthält keine `conf/resource` und fordert keine Ressourcen-Worker an. Die aktuelle Desktop-Benachrichtigung verwendet App-Sprachtexte und benötigt dafür keine `sysnotify`-Registrierung. Der Builder prüft diese Paketkonfiguration. Die zuvor zugeordnete Installationsmeldung gehörte zu einem anderen Paket; eine Ablehnung von `sysnotify` für SynoWake wurde damit nicht nachgewiesen.

Für Desktop-Meldungen verwendet der Dienst `synodsmnotify -c SYNO.SDS.SynoWake.Application -p plain @administrators SynoWake:notification:title <Nachrichtenschlüssel> <Gerätename>`. Die Sprachdateien `ui/texts/{ger,enu}/strings` enthalten Titel, eine allgemeine Meldung und die Vorlagen `wake_sent` sowie `wake_failed`. DSM löst die Vorlage in der Sprache des Empfängers auf. Nur der unveränderte Gerätename wird als eigenes Prozessargument in `{0}` eingesetzt; Fehlerdetails stehen im Anwendungsprotokoll. `texts` und alle vier `preloadTexts` sind im App-Config eingetragen, damit Meldungen auch bei geschlossenem App-Fenster möglich sind. Es wird keine Shell oder ein JSON-Mailstring verwendet. Fehler erscheinen als Diagnose und lokaler Protokolleintrag. Gestoppte Zeitpläne senden weiterhin weder Magic Packet noch Benachrichtigung.

## Quellen

- [Synology INFO-Felder](https://help.synology.com/developer-guide/synology_package/INFO_optional_fields.html): `dsmuidir`, `dsmappname`, Prüfsumme und Paketsteuerung.
- [Synology Privilege Config](https://help.synology.com/developer-guide/privilege/privilege_config.html): Paketidentität und ausführbare Dateien.
- [Synology FHS](https://help.synology.com/developer-guide/integrate_dsm/fhs.html): separate `target`- und `var`-Verzeichnisse.
- [Synology Application Authentication](https://help.synology.com/developer-guide/integrate_dsm/web_authentication.html): Prüfung der DSM-Sitzung aus einer Paket-CGI.
- [Synology Desktop Notifications](https://help.synology.com/developer-guide/synology_package/show_massage.html), [Application I18N](https://help.synology.com/developer-guide/integrate_dsm/i18n.html) und [App-Config](https://help.synology.com/developer-guide/integrate_dsm/config.html): App-I18N-Schlüssel, Sprachdateien und `preloadTexts`.
- [Synology Log Receiving](https://kb.synology.com/index.php/en-us/DSM/help/LogCenter/logcenter_server?version=7): Protokoll-Center-Empfänger mit BSD-Format, TCP und frei wählbarem Port.
- [AutoPilot config](https://github.com/toafez/AutoPilot/blob/main/ui/config) und [AutoPilot.js](https://github.com/toafez/AutoPilot/blob/main/ui/AutoPilot.js): eigener Quelltext als Beispiel des DSM-Anwendungsfensters.
- [N4S4 Task Scheduler](https://github.com/N4S4/synology-api/blob/master/synology_api/task_scheduler.py) und [WebAPI-Transport](https://github.com/N4S4/synology-api/blob/master/synology_api/auth.py): primärer Projektquelltext einer inoffiziellen API-Implementierung, als Referenz für Aufgabenfelder und Request-Format.
