# SynoWake

Wake-on-LAN-Anwendung für eine Synology **DS918+ mit DSM 7.1**. Der Paketname und der Name im DSM lauten **SynoWake**.

Die Oberfläche öffnet ein kleines DSM-Anwendungsfenster. Sie bietet Gerätekacheln mit Schnellauswahl, eine bearbeitbare Geräteliste mit Netzwerksuche und einen Tab für Zeitpläne mit Ausführungsprotokoll. Zeitpläne werden über die vorhandene DSM-Sitzung in `SYNO.Core.TaskScheduler` angelegt, geändert und gelöscht. Die Anwendung schreibt weder `/etc/crontab` noch `task_config.xml`.

**Version 0.1.4-0005: Styles vom DSM-Desktop getrennt, Beta mit Zielsystemprüfung.** Die Oberfläche verwendet ausschließlich `ui/assets/synowake.css` innerhalb ihres iframe. Alle CSS-Selektoren sind auf `#synowake-app` begrenzt; eine globale `ui/style.css` wird nicht mehr ausgeliefert. Dadurch sollen Paket-Zentrum, Widgets und andere DSM-Anwendungen beim Aktivieren von SynoWake unverändert bleiben. Nach dem Update DSM neu laden, damit eventuell bereits geladene alte CSS-Regeln verschwinden. Der Builder weist die alte globale CSS-Datei ab. Die Korrektur wird mit einem Browser-Test für unveränderte Stile und Abmessungen außerhalb von SynoWake geprüft; die tatsächliche DSM-Integration bleibt auf der NAS zu bestätigen.

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

Das Paket ist auf die DSM-Architektur `apollolake` und mindestens `7.1-42661` eingestellt. Es ist nicht signiert und wird als Beta gekennzeichnet.

## Installation und erste Verwendung

1. Im DSM mit einem Administratorkonto anmelden.
2. **Paket-Zentrum → Manuelle Installation** öffnen und die erzeugte `.spk` auswählen. DSM zeigt bei diesem privaten Paket eine Warnung zum Herausgeber; Inhalt und Herkunft vor dem Fortfahren prüfen.
3. Paket starten und **SynoWake** über das Hauptmenü oder die Schaltfläche **Öffnen** aufrufen.
4. Im Tab **Geräte** ein Gerät mit Name, IPv4-LAN-Adresse und MAC-Adresse anlegen. Private Adressen sowie Adressen aus direkt angeschlossenen NAS-Netzen werden akzeptiert. Der Name bleibt bearbeitbar. Broadcast-Adresse und UDP-Port können bei Bedarf angepasst werden.
5. Wake-on-LAN im BIOS/UEFI, Betriebssystem und Netzwerktreiber des Zielgeräts aktivieren. Zunächst im gleichen kabelgebundenen LAN testen.
6. Auf der Startseite auf eine Gerätekachel klicken oder mehrere Geräte über die Schnellauswahl aufwecken.
7. Im Tab **Automatik** Gerät, Uhrzeit, Wochentage und optional DSM-Benachrichtigungen wählen. Die Uhrzeit folgt der Zeitzone der NAS.

Wenn das DSM-Anwendungsfenster auf der konkreten Firmware nicht lädt, die Oberfläche nach DSM-Anmeldung direkt unter `https://NAS:DSM-Port/webman/3rdparty/SynoWake/index.html` öffnen. Für einen eigenen HTTPS-Port dessen Wert einsetzen. Das ist zugleich ein Diagnoseweg für die Fensterintegration.

## Status und Netzwerksuche

`Online` bedeutet, dass die NAS eine ICMP-Antwort erhält. `Offline` bedeutet, dass derzeit keine solche Antwort vorliegt; eine Firewall kann einen laufenden Rechner ebenfalls so erscheinen lassen. Nach einem gesendeten Magic Packet zeigt SynoWake zunächst `Wird aufgeweckt` und prüft erneut. Das Senden allein beweist keinen erfolgreichen Start.

Beim Öffnen der Suche ermittelt SynoWake die aktiven IPv4-Netze der NAS und füllt den Suchbereich automatisch aus. Mehrere Schnittstellen sind auswählbar. Bevorzugt wird die Schnittstelle, über deren Adresse die DSM-Anfrage eingeht. Ein festes `192.168.1.0/24` oder die Adresse des ersten gespeicherten Geräts wird nicht als Standard verwendet. Loopback, nicht aktive Schnittstellen und `169.254.*` werden nicht angeboten.

Die Suche akzeptiert `/24` bis `/30`, vollständig innerhalb eines angeschlossenen Schnittstellennetzes. Direkt angeschlossene Netze wie `192.167.178.0/24` werden ebenfalls akzeptiert. Kleinere Teilnetze behalten die echte Netzmaske; bei größeren Netzen wird zunächst der `/24`-Bereich mit der NAS-Adresse vorgeschlagen. Weitere passende Teilbereiche lassen sich eingeben. Die Begrenzung liegt bei 254 Hostadressen pro Suche.

Die Suche prüft Hosts und liest Nachbartabellen. IP, MAC und die Broadcast-Adresse der zugehörigen NAS-Schnittstelle lassen sich aus Treffern übernehmen. Damit funktioniert auch das Speichern eines Geräts aus einem angeschlossenen Netz außerhalb der privaten IPv4-Bereiche. Wenn Reverse-DNS einen Namen liefert, wird dieser vorgeschlagen; sonst erscheint ein allgemeiner LAN-Gerätetyp. Namen können jederzeit angepasst werden.

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

Ist bei einem Zeitplan der Benachrichtigungshaken gesetzt, sendet `synodsmnotify` eine DSM-Desktopmeldung an die Administratorgruppe. Titel und Nachricht verwenden App-I18N-Schlüssel aus `ui/texts/{ger,enu}/strings`; `preloadTexts` lädt diese auch bei geschlossenem SynoWake-Fenster. Der Ausführungstext wird als einzelnes Argument in den Platzhalter `{0}` eingesetzt. Eine `sysnotify`-Ressourcenregistrierung ist dafür nicht erforderlich. Benachrichtigungsfehler werden im Anwendungsprotokoll angezeigt.

Die Schritte für die Prüfung auf der NAS stehen in [docs/DSM-TEST.md](docs/DSM-TEST.md). Hinweise zur DSM-Anbindung stehen in [docs/ARCHITEKTUR.md](docs/ARCHITEKTUR.md).

## Quellen der DSM-Anbindung

Die Paketstruktur und Identitäten folgen dem [Synology Package Developer Guide](https://help.synology.com/developer-guide/), insbesondere [INFO](https://help.synology.com/developer-guide/synology_package/INFO.html), [Privilege Config](https://help.synology.com/developer-guide/privilege/privilege_config.html), [Application Authentication](https://help.synology.com/developer-guide/integrate_dsm/web_authentication.html), [Desktop Notifications](https://help.synology.com/developer-guide/synology_package/show_massage.html) und [Application I18N](https://help.synology.com/developer-guide/integrate_dsm/i18n.html).

Der aktuelle Guide beschreibt DSM 7.2.2. Die Anwendung zielt auf 7.1; aktuelle Dokumentation ersetzt deshalb keinen Test auf der gewünschten Version. Für das native DSM-Fenster dient die vom Entwickler veröffentlichte [AutoPilot-Fensterintegration](https://github.com/toafez/AutoPilot/blob/main/ui/AutoPilot.js) als Implementierungsbeispiel. Für Aufgabenobjekte und Methoden dient der eigene Quelltext von [N4S4/synology-api – Task Scheduler](https://github.com/N4S4/synology-api/blob/master/synology_api/task_scheduler.py) sowie dessen [WebAPI-Transport](https://github.com/N4S4/synology-api/blob/master/synology_api/auth.py) als Referenz. Dieses Projekt ist eine inoffizielle Implementierung; der Aufgabenplaner bleibt eine interne DSM-API, deren konkrete Antwortform auf dem Zielsystem geprüft werden muss.
