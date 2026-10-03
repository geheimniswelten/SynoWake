# SynoWake

Deutsch · [English](README.en.md)

Wake-on-LAN-Anwendung für eine Synology **DS918+ mit DSM 7.1**. Der Paketname und der Name im DSM lauten **SynoWake**.

Die Oberfläche öffnet ein kleines DSM-Anwendungsfenster. Sie bietet Gerätekacheln mit Schnellauswahl, eine bearbeitbare Geräteliste mit Netzwerksuche und einen Tab für Zeitpläne mit Ausführungsprotokoll. Zeitpläne werden über die vorhandene DSM-Sitzung in `SYNO.Core.TaskScheduler` angelegt, geändert und gelöscht. Die Anwendung schreibt weder `/etc/crontab` noch `task_config.xml`.

**Version 0.1.13-0014: kurze WoL-Pausen und vollständiges Deutsch/Englisch.** Jede Weckaktion sendet drei identische Magic Packets mit jeweils 20 ms Pause dazwischen. Der gesamte Versand bleibt eine Weckaktion mit einem gemeinsamen Ausführungseintrag. Es gibt keinen zusätzlichen Versand aufgrund ausbleibender Statusantworten.

**Sprachen:** Oberfläche, Formulare, Status, Validierungen, DSM-Aufgabenplaner-Meldungen, Suchhinweise, Protokolltexte und Benachrichtigungsvorlagen sind auf Deutsch und Englisch vorhanden. Die Oberfläche bevorzugt `SYNO.SDS.Session.lang` der DSM-Sitzung. Ohne erreichbare DSM-Sprachangabe folgt sie der Browsersprache; andere Sprachen verwenden Englisch. Für die direkte Vorschau kann `?lang=de` oder `?lang=en` verwendet werden. Der Browser teilt seine Auswahl dem Backend über `Accept-Language` mit. Gespeicherte Geräte- und Aufgabennamen sowie DNS-Namen bleiben erhalten; nur Demo-Namen und automatisch erzeugte Vorschläge werden übersetzt. Neue Protokolle bewahren Vorlage und Argumente getrennt, damit dieselbe Aktion in beiden Sprachen angezeigt werden kann. Unveränderte technische Details des Betriebssystems bleiben als Diagnose erhalten. Lebenszyklus- und Protokoll-Center-Texte verwenden `SYNOPKG_DSM_LANGUAGE` des Paketprozesses, ersatzweise Englisch.

Die Paketangaben bleiben erhalten: Entwickler `himitsu` mit `https://geheimniswelten.de`, Herausgeber `geheimniswelten` mit `https://github.com/geheimniswelten/SynoWake`, Support unter dessen Issues-Seite. Die GitHub-Adressen sind für das noch anzulegende öffentliche Projekt vorbereitet. `report_url` entfällt; `thirdparty="yes"` bleibt gesetzt. Die Angaben bestimmen keine DSM-Benutzerrechte.

`thirdparty` kennzeichnet historisch Pakete anderer Entwickler. Laut [Synology-INFO-Dokumentation](https://help.synology.com/developer-guide/synology_package/INFO_optional_fields.html) wird das Feld seit DSM 5.0 nicht mehr verwendet; für DSM 7.1 entscheidet es daher nicht über den Veröffentlichungsweg. Die Aufnahme ins offizielle Paket-Zentrum erfordert eine eigene Bewerbung und Paketprüfung.

Die automatischen Namen aus 0.1.10 bleiben erhalten: Das Namensfeld ist optional. Bleibt es leer oder enthält nur Leerzeichen, verwendet SynoWake Gerätename, ausgewählte Tage und Uhrzeit, beispielsweise `ACER-Frank · Mo–Fr · 08:00`. Alle Tage erscheinen als `Täglich`, einzelne Tage in der Reihenfolge Montag bis Sonntag. Eigene Namen bleiben erhalten. Bei langen Gerätenamen wird nur der Gerätenamenanteil gekürzt, damit Tage und Uhrzeit innerhalb der 80-Zeichen-Grenze erhalten bleiben.

Die Aufgabenanlage über die native DSM-Sitzungsanbindung aus 0.1.9 wurde vom Benutzer auf der Ziel-NAS bestätigt. Im DSM-Fenster verwendet SynoWake `SYNO.API.Request` des übergeordneten DSM-Dokuments. Diese übernimmt den aktuellen Sitzungsschutz einschließlich `X-SYNO-HASH`. Ein Fehler 105 wird als Ablehnung der Sitzung beschrieben und nicht pauschal als fehlende Administratoranmeldung.

Die Versionswahl aus 0.1.8 bleibt erhalten: SynoWake bevorzugt die Katalogversion und berücksichtigt für get/create/set die Versionen 2 bis 4. Nur bei einer ausdrücklichen Versionsablehnung (104) wird eine andere Version versucht; erfolgreiche Versionen werden pro Methode für die laufende Oberfläche gemerkt. Verbindungsfehler, unklare Antworten und andere API-Fehler führen zu keiner automatischen Wiederholung. Ein unterbrochener nativer DSM-Aufruf wechselt nicht zum direkten Aufruf. Bei gesperrtem ICMP bleibt die TCP-Erreichbarkeitsprüfung erhalten.

Bereits gespeicherte Geräte bleiben in der Suche sichtbar und erhalten die Markierung **Bereits vorhanden**. Ihr gespeicherter Name wird angezeigt; Übernahme-Häkchen und Namensfeld sind gesperrt. Der Vergleich verwendet die MAC-Adresse und funktioniert auch bei geänderter IP. Vor der Übernahme wird der Bestand erneut abgefragt; das Backend verhindert zusätzlich neue Einträge mit einer bereits gespeicherten MAC-Adresse.

Die Favoriten-Auswahl aus 0.1.5 bleibt erhalten: Nur markierte Geräte erscheinen als Kacheln. Neue Suchtreffer beginnen ohne Häkchen. Die Suche merkt sich den zuletzt erfolgreich verwendeten Bereich und schlägt sonst das aktive NAS-Netz vor. Bekannte lokale DNS-Endungen werden aus Namensvorschlägen entfernt. Die Oberflächendateien verwenden URLs mit Paketversion. Nach dem Update DSM vollständig neu laden, beim beschriebenen Firefox-Ablauf mit **Shift+Klick auf den Aktualisieren-Knopf**.

Die Oberfläche verwendet weiterhin ausschließlich `ui/assets/synowake.css` innerhalb ihres iframe. Alle CSS-Selektoren sind auf `#synowake-app` begrenzt. Der Builder weist eine globale `ui/style.css` ab. Ein Browser-Test prüft unveränderte Stile und Abmessungen außerhalb von SynoWake; die tatsächliche DSM-Integration bleibt auf der NAS zu bestätigen.

Die Suche verwendet aktive NAS-Schnittstellen und deren Netzmasken. Auch direkt angeschlossene Netze außerhalb der privaten IPv4-Bereiche werden unterstützt. Der Paketdienst verwendet ausschließlich das Paketkonto. Die CGI-Weiterleitung benötigt keine erhöhten Ausführungsrechte. Das Paket fordert keine DSM-Ressourcen an; Desktop-Benachrichtigungen verwenden App-Sprachtexte. Netzwerkverkehr, Aufgabenplaner, Protokoll-Center und Benachrichtigungen müssen auf der tatsächlichen NAS bestätigt werden.

## Paket bauen

Auf dem Entwicklungsrechner werden **Python 3.10 oder neuer** und **Go 1.24 oder neuer** benötigt. Die NAS benötigt weder Go noch Python, PHP, Web Station oder Container Manager: Das Paket enthält ein statisches Linux/amd64-Programm und statische Oberflächendateien.

```powershell
python scripts/build.py
```

Alternativ ein bereits gebautes statisches Programm verwenden:

```powershell
python scripts/build.py --binary build/synowake --output dist/SynoWake.spk
```

Der Builder prüft das ELF-Ziel Linux/amd64 und lehnt dynamisch gelinkte Programme ab. Er erzeugt ein SPK-Archiv, eine SHA256-Datei und den von DSM erwarteten MD5-Prüfwert für `package.tgz`. Archivpfade, Reihenfolge, Zeitstempel, Zeilenenden und POSIX-Dateirechte werden festgelegt; beim gleichen Eingabebestand entstehen identische Archive.

CSS-Regression prüfen: `node scripts/test-ui.mjs` starten und die ausgegebene lokale Adresse in Firefox öffnen. Der Test reproduziert die Änderung am äußeren Dokument mit der alten CSS, prüft die neue Begrenzung, Dialoge und vier Fensterbreiten. Die alte CSS liegt ausschließlich unter `tests/fixtures` und wird nicht ins SPK aufgenommen.

Den gesamten Oberflächenablauf einschließlich Favoriten, Geräteübernahme und simulierten DSM-WebAPI-Antworten prüft `node tests/workflow.cjs` mit Playwright. `PLAYWRIGHT_MODULE` kann den Pfad zur installierten Playwright-Bibliothek und `BROWSER_EXECUTABLE` den Pfad zu Chromium oder Edge vorgeben. Screenshots und CSS-Prüfergebnis werden unter `work/` gespeichert. Backend-Tests: `go test ./...`.

`node tests/scheduler.cjs` prüft außerdem Methodenaufrufe trotz Katalogversion 3, API-Erkennung über moderne und ältere Endpunkte, Fehlercodes und sichere Wiederaufnahme unterbrochener Aufgabenplaner-Anfragen. Die native DSM-Anbindung wird mit typisierten Parametern, Versionsablehnung, Berechtigungsfehler, Verbindungsabbruch, Timeout und verspäteter Antwort geprüft. Der Browser-Test prüft Aufgabenanlage, Änderungen, Abgleich und Löschen über eine nachgebildete DSM-Anfragefunktion sowie deren Zugriff aus einem eingebetteten iframe.

`node tests/i18n.cjs` und `python tests/check-i18n.py` prüfen die Sprachwahl, Parameter der Übersetzungen, sämtliche statischen Oberflächentexte, Benachrichtigungskataloge und sprachunabhängige Fehlerbehandlung. Der Browser-Test prüft auch den englischen Ablauf. Der gemeinsame Übersetzungskatalog liegt in `internal/synowake/translations.json`: Go bettet ihn ein; der Builder erzeugt daraus `ui/translations.js` für den Browser. Der Builder kontrolliert die Parameter der Vorlagen und die Versionsangaben der Sprachmodule.

Das Paket ist auf die DSM-Architektur `apollolake` und mindestens `7.1-42661` eingestellt. Es ist nicht signiert und wird als Beta gekennzeichnet.

## Installation und erste Verwendung

1. Im DSM mit einem Administratorkonto anmelden.
2. **Paket-Zentrum → Manuelle Installation** öffnen und die erzeugte `.spk` auswählen. DSM zeigt bei diesem privaten Paket eine Warnung zum Herausgeber; Inhalt und Herkunft vor dem Fortfahren prüfen.
3. Paket starten und **SynoWake** über das Hauptmenü oder die Schaltfläche **Öffnen** aufrufen.
4. Im Tab **Geräte** ein Gerät mit Name, IPv4-LAN-Adresse und MAC-Adresse anlegen. Private Adressen sowie Adressen aus direkt angeschlossenen NAS-Netzen werden akzeptiert. Der Name bleibt bearbeitbar. Broadcast-Adresse und UDP-Port können bei Bedarf angepasst werden.
5. Wake-on-LAN im BIOS/UEFI, Betriebssystem und Netzwerktreiber des Zielgeräts aktivieren. Zunächst im gleichen kabelgebundenen LAN testen.
6. In der Geräteliste die gewünschten Geräte als **Favorit** markieren oder beim Bearbeiten **Als Kachel in Übersicht anzeigen** aktivieren. Auf der Startseite eine dieser Kacheln oder die Schnellauswahl verwenden. Alle Geräte bleiben unabhängig vom Favoritenstatus in der Liste und für Automatiken verfügbar.
7. Im Tab **Automatik** Gerät, Uhrzeit, Wochentage und optional DSM-Benachrichtigungen wählen. Die Uhrzeit folgt der Zeitzone der NAS.

Wenn das DSM-Anwendungsfenster auf der konkreten Firmware nicht lädt, die Oberfläche nach DSM-Anmeldung direkt unter `https://NAS:DSM-Port/webman/3rdparty/SynoWake/index.html` öffnen. Für einen eigenen HTTPS-Port dessen Wert einsetzen. Das ist zugleich ein Diagnoseweg für die Fensterintegration.

Für Automatiken SynoWake aus dem DSM-Hauptmenü öffnen. Bei einer direkt geöffneten Oberfläche fehlt die übergeordnete DSM-Anfragefunktion; der direkte WebAPI-Ersatz kann deshalb je nach DSM-Sitzungsschutz abgelehnt werden.

## Status und Netzwerksuche

`Online` bedeutet, dass die NAS eine ICMP-Antwort erhält oder bei nicht verfügbarem ICMP eine TCP-Verbindung beziehungsweise eine ausdrückliche Verbindungsablehnung feststellt. Letztere zeigt ebenfalls eine IP-Antwort. `Offline` bedeutet bei nutzbarem ICMP, dass keine Antwort vorliegt; eine Firewall kann einen laufenden Rechner ebenfalls so erscheinen lassen. Ist ICMP nicht verfügbar und bleibt auch TCP ohne Antwort, erscheint **Unbekannt**. Nach einem gesendeten Magic Packet zeigt SynoWake zunächst bis zu 90 Sekunden `Wird aufgeweckt` und prüft erneut. Das Senden allein beweist keinen erfolgreichen Start.

Der TCP-Ersatz prüft die Ports 445, 80, 443, 22, 3389 und 5000 parallel mit insgesamt 800 ms Frist pro Gerät. Es werden keine Anmeldedaten oder Anwendungsbefehle gesendet. Die Suche meldet die Verwendung dieses Ersatzwegs; die Statusanzeige erklärt ihn beim Überfahren mit der Maus. Raw-Socket-Rechte, Root-Ausführung, Datei-Capabilities und Änderungen an DSM-Kernelparametern werden nicht angefordert. Vollständig gefilterte Geräte können nicht eindeutig als ausgeschaltet erkannt werden.

Beim Öffnen der Suche ermittelt SynoWake die aktiven IPv4-Netze der NAS und füllt den Suchbereich automatisch aus. Mehrere Schnittstellen sind auswählbar. Bevorzugt wird die Schnittstelle, über deren Adresse die DSM-Anfrage eingeht. Ein festes `192.168.1.0/24` oder die Adresse des ersten gespeicherten Geräts wird nicht als Standard verwendet. Loopback, nicht aktive Schnittstellen und `169.254.*` werden nicht angeboten.

Nach einer erfolgreichen Suche wird der verwendete Bereich in den Paketdaten gespeichert und beim nächsten Öffnen wieder vorgeschlagen, auch nach einem Browser- oder Paketneustart. Liegt er inzwischen nicht mehr in einem aktiven NAS-Netz, wird stattdessen ein aktuelles Netz angeboten. Bei fehlender Netzwerkerkennung bleibt das Feld leer. Unterschiedliche Versionsstände von Oberfläche und Paketdienst erzeugen eine sichtbare Meldung.

Die Suche akzeptiert `/24` bis `/30`, vollständig innerhalb eines angeschlossenen Schnittstellennetzes. Direkt angeschlossene Netze wie `192.167.178.0/24` werden ebenfalls akzeptiert. Kleinere Teilnetze behalten die echte Netzmaske; bei größeren Netzen wird zunächst der `/24`-Bereich mit der NAS-Adresse vorgeschlagen. Weitere passende Teilbereiche lassen sich eingeben. Die Begrenzung liegt bei 254 Hostadressen pro Suche.

Die Suche prüft Hosts und liest Nachbartabellen. IP, MAC und die Broadcast-Adresse der zugehörigen NAS-Schnittstelle lassen sich aus ausgewählten neuen Treffern übernehmen. Zu Beginn ist kein Treffer angehakt. Bereits gespeicherte MAC-Adressen werden als **Bereits vorhanden** markiert und können nicht erneut übernommen werden; ihr gespeicherter Name bleibt erhalten. Übernommene Geräte sind zunächst keine Favoriten. Wenn Reverse-DNS einen Namen liefert, werden `.fritz.box`, `.local` und `.lan` am Ende entfernt, beispielsweise `ACER-Frank.fritz.box` → `ACER-Frank`; sonst erscheint ein allgemeiner LAN-Gerätetyp. Das Feld heißt „Name“ und bleibt bei neuen Treffern bearbeitbar. Bereits gespeicherte oder selbst eingegebene Namen werden nicht automatisch umbenannt.

Schlafende Geräte, entfernte VLANs und Hosts ohne verwertbaren Nachbartabelleneintrag können fehlen. Solche Geräte manuell eintragen. Eine IP-Fixierung bzw. DHCP-Reservierung verhindert, dass ein gespeichertes Gerät später eine andere Adresse erhält.

## Zeitpläne und Paketlebenszyklus

Zeitpläne erscheinen zusätzlich im DSM-Aufgabenplaner als SynoWake-Aufgaben. Erstellen, Bearbeiten und Löschen benötigt eine laufende DSM-Administratorsitzung. Die Browseroberfläche verwendet die interne Aufgabenplaner-WebAPI; Passwörter werden dabei nicht in SynoWake gespeichert.

Aufgaben rufen das mitgelieferte CLI auf, das eine auf den lokalen DSM-Endpunkt begrenzte Anfrage an den CGI-Endpunkt sendet. Dieser leitet sie über einen lokalen Unix-Socket an den Paketdienst weiter. Jeder Zeitplan erhält ein eigenes Geheimnis. Der Dienst und die Lebenszyklusbefehle lesen und schreiben die privaten Daten in `/var/packages/SynoWake/var` unter dem Paketkonto; ausführbare Dateien liegen unter `/var/packages/SynoWake/target`. Die Berechtigungskonfiguration enthält ausschließlich `defaults.run-as: package`. Alle Programmdateien haben Modus `0755`; zusätzliche Ausführungsprivilegien, Setuid, Gruppenänderungen und Datei-Capabilities werden nicht angefordert.

**Paket stoppen** deaktiviert die Ausführung und beendet den Dienst. Bereits registrierte DSM-Aufgaben bleiben vorhanden, dürfen aber im gestoppten Zustand kein Gerät aufwecken. **Paket starten** startet den Dienst und prüft seine Erreichbarkeit. Ein Upgrade verwendet die vorhandenen Paketdaten. Startprobleme werden in `/var/packages/SynoWake/var/service.log` protokolliert.

**Vor der Deinstallation alle SynoWake-Zeitpläne in der Anwendung entfernen.** Die Paket-Skripte besitzen keine DSM-Administratorsitzung und löschen Aufgaben deshalb nicht eigenmächtig. Eventuell verbliebene Aufgaben im DSM-Aufgabenplaner anhand des SynoWake-Präfixes kontrollieren und löschen. Nach dem Entfernen des Pakets enthalten sie einen nicht mehr vorhandenen Programmpfad.

## Protokolle und Benachrichtigungen

Jede ausgeführte Wake-Aktion wird im SynoWake-Protokoll gespeichert. Zusätzlich versucht die Anwendung, einen DSM-Systemeintrag über `synologset1` zu schreiben. Wenn DSM diese interne Funktion für das Paketkonto sperrt, lässt sich im Automatik-Tab ein lokaler TCP-Syslog-Empfänger des Protokoll-Centers konfigurieren. Fehler der DSM-Anbindung erscheinen als Diagnose in SynoWake; nicht übertragene Einträge bleiben für einen erneuten Versand vorgemerkt. Die Sichtbarkeit im **Protokoll-Center** muss auf DSM 7.1 geprüft werden, bevor sie als erfüllt gelten kann.

SynoWake bewahrt alle noch nicht übertragenen Einträge und die neuesten 300 übertragenen Einträge auf. Bei 3000 ausstehenden Einträgen hält die Anwendung weitere Wake-Aktionen an, bis die Protokoll-Center-Anbindung funktioniert und der Rückstand übertragen wurde. Ein Ausführungseintrag wird bereits vor dem Magic Packet gespeichert und anschließend mit dem Ergebnis ergänzt, damit ein Prozessabbruch nicht die gesamte Aufzeichnung verliert.

Für diesen Ersatzweg zunächst im vollständigen **Protokoll-Center → Archiveinstellungen** ein Speicherziel wählen. Unter **Protokolle empfangen → Erstellen** das Format **BSD (RFC 3164)**, **TCP** und einen freien Empfangsport wählen, beispielsweise **514**. SSL für diesen lokalen Sender ausschalten. In SynoWake denselben Port eintragen und speichern; die Anwendung sendet ausschließlich an `127.0.0.1`. Wenn DSM-Firewallregeln greifen, muss der lokale Zugriff auf diesen Port möglich sein. Danach eine Wake-Aktion ausführen, den Eintrag im Protokoll-Center prüfen und bei Bedarf die vorgemerkten Einträge erneut senden. Die Einrichtung folgt der [Synology-Anleitung für den Log-Empfang](https://kb.synology.com/index.php/en-us/DSM/help/LogCenter/logcenter_server?version=7). Der Empfangsport ist anfänglich deaktiviert (`0`).

Ist bei einem Zeitplan der Benachrichtigungshaken gesetzt, sendet `synodsmnotify` eine DSM-Desktopmeldung an die Administratorgruppe. Titel und Nachricht verwenden App-I18N-Schlüssel aus `ui/texts/{ger,enu}/strings`; `preloadTexts` lädt diese auch bei geschlossenem SynoWake-Fenster. DSM übersetzt die Erfolg- oder Fehlervorlage für den jeweiligen Empfänger; der Gerätename wird als einzelnes unverändertes Argument in `{0}` eingesetzt. Bei Fehlern verweist die Meldung auf das SynoWake-Protokoll mit den technischen Details. Eine `sysnotify`-Ressourcenregistrierung ist dafür nicht erforderlich. Benachrichtigungsfehler werden im Anwendungsprotokoll angezeigt.

Die Schritte für die Prüfung auf der NAS stehen in [docs/DSM-TEST.md](docs/DSM-TEST.md). Hinweise zur DSM-Anbindung stehen in [docs/ARCHITEKTUR.md](docs/ARCHITEKTUR.md).

## Quellen der DSM-Anbindung

Die Paketstruktur und Identitäten folgen dem [Synology Package Developer Guide](https://help.synology.com/developer-guide/), insbesondere [INFO](https://help.synology.com/developer-guide/synology_package/INFO.html), [Privilege Config](https://help.synology.com/developer-guide/privilege/privilege_config.html), [Application Authentication](https://help.synology.com/developer-guide/integrate_dsm/web_authentication.html), [Desktop Notifications](https://help.synology.com/developer-guide/synology_package/show_massage.html) und [Application I18N](https://help.synology.com/developer-guide/integrate_dsm/i18n.html).

Der aktuelle Guide beschreibt DSM 7.2.2. Die Anwendung zielt auf 7.1; aktuelle Dokumentation ersetzt deshalb keinen Test auf der gewünschten Version. Für das native DSM-Fenster dient die vom Entwickler veröffentlichte [AutoPilot-Fensterintegration](https://github.com/toafez/AutoPilot/blob/main/ui/AutoPilot.js) als Implementierungsbeispiel. Für Aufgabenobjekte und Methoden dient der eigene Quelltext von [N4S4/synology-api – Task Scheduler](https://github.com/N4S4/synology-api/blob/master/synology_api/task_scheduler.py) sowie dessen [WebAPI-Transport](https://github.com/N4S4/synology-api/blob/master/synology_api/auth.py) als Referenz. Dieses Projekt ist eine inoffizielle Implementierung; der Aufgabenplaner bleibt eine interne DSM-API, deren konkrete Antwortform auf dem Zielsystem geprüft werden muss.
