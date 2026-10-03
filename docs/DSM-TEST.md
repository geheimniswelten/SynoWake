# Abnahme auf DS918+ / DSM 7.1

Deutsch · [English](DSM-TEST.en.md)

Diese Liste ist eine Anleitung für die Zielsystemprüfung. Ein lokaler Build oder eine Browser-Demo bestätigt keine DSM-Installation. Vor dem Test die tatsächliche DSM-Version und die NAS-Zeitzone notieren. Ein einziger Testrechner mit bekannter MAC-Adresse und funktionierendem Wake-on-LAN genügt für den ersten Durchlauf.

| Prüfung | Erwartetes Ergebnis |
| --- | --- |
| SPK manuell installieren | Paket-Zentrum akzeptiert die Architektur und Mindestversion; keine Root-Privilege-Meldung und keine abgelehnte Synology-Ressource. |
| Installierte Programmrechte prüfen | `ui/api.cgi` und `bin/synowake` haben Modus `0755`; keine Setuid-/Setgid-Bits und keine zusätzlichen Ausführungsprivilegien. |
| Paketdienst prüfen | Dienst läuft unter dem SynoWake-Paketkonto; `target/run/backend.sock` existiert und `bin/synowake status` liefert 0. |
| Paket starten und öffnen | Eigenes DSM-Fenster zeigt Gerätekacheln, Geräteliste und Automatik. |
| DSM-Sprache Deutsch und Englisch prüfen | Sämtliche Tabs, Dialoge, Status, Suchhinweise, Fehler und Protokolle folgen der DSM-Sprache. Vorhandene Namen bleiben unverändert. |
| Oberfläche direkt mit deutscher/englischer Browsersprache öffnen | Ohne DSM-Sprachangabe gilt die Browsersprache; andere Sprachen fallen auf Englisch zurück. |
| Zugriff ohne DSM-Anmeldung | `api.cgi` verweigert Daten und Wake-Aktionen. |
| Zugriff als normaler DSM-Benutzer | Backend verweigert die Administrationsfunktionen. |
| Gerät mit Name, IP und MAC speichern | Zeile erscheint ohne Favoriten-Häkchen; Name lässt sich ändern. |
| Favorit setzen und entfernen | Nur markierte Geräte erscheinen als Kacheln; Auswahl bleibt nach Neuladen und Paketneustart gespeichert. Alle Geräte bleiben in der Liste und für Automatiken verfügbar. |
| Nur Favoriten mit „Alle auswählen“ aufwecken | Nur sichtbare Kacheln werden aufgeweckt. Beim Entfernen eines Favoriten verschwindet er auch aus der Schnellauswahl. |
| Falsche IP oder MAC eingeben | Sichtbare Validierungsfehlermeldung; kein Wake-Paket. |
| Suchdialog ohne vorhandene Geräte öffnen | Suchbereich wird aus dem aktiven NAS-Netz vorgeschlagen; kein festes `192.168.1.0/24`. |
| Mehrere aktive Schnittstellen prüfen | NAS-IP, Schnittstelle und echtes Netz sind auswählbar; nicht aktive, Loopback- und `169.254.*`-Schnittstellen fehlen. |
| Lokales `/24` suchen, auch `192.167.178.0/24` | Erreichbare LAN-Nachbarn mit MAC erscheinen; Treffer samt lokalem Broadcast lassen sich übernehmen und speichern. |
| Suchtreffer übernehmen | Alle Häkchen sind zunächst leer. Nur angehakte Treffer werden übernommen und beginnen ohne Favoriten-Häkchen. |
| Gespeicherte Geräte erneut suchen | Treffer bleiben sichtbar, zeigen „Bereits vorhanden“ und ihren gespeicherten Namen. Häkchen und Namensfeld sind gesperrt; es entsteht kein doppelter Eintrag. Eine geänderte IP bei gleicher MAC ändert diese Erkennung nicht. |
| Namen in der Suche prüfen | Feld heißt „Name“; beispielsweise `ACER-Frank.fritz.box` wird als `ACER-Frank` vorgeschlagen und bleibt bearbeitbar. |
| Erfolgreich eingegebenen Suchbereich wieder öffnen | Nach DSM-Neuladen oder Paketneustart wird der gespeicherte Bereich wieder angeboten, sofern er noch innerhalb eines aktiven NAS-Netzes liegt. |
| Tatsächliches `/25` bis `/30` prüfen | Suchvorschlag behält die Netzmaske; über das angeschlossene Netz hinausgehende Bereiche werden abgewiesen. |
| Einzelgerät und Schnellauswahl starten | Drei identische Magic Packets mit jeweils etwa 20 ms Pause erreichen jeden Testrechner. Genau ein Ausführungseintrag pro Gerät; keine statusabhängige Wiederholung. Status wechselt von „Wird aufgeweckt“ nach „Online“, wenn ICMP oder der TCP-Ersatz eine Antwort nachweist. |
| ICMP-Ausführung für das Paketkonto gesperrt | Suche verwendet TCP, füllt Nachbartabellen und erklärt den Ersatzweg. Antworten auf Verbindungsaufbau oder ausdrückliche Ablehnung ergeben „Online“. Keine Antwort ergibt „Unbekannt“. |
| ICMP nutzbar, Ziel antwortet nicht | Wie bisher „Offline“ als fehlende ICMP-Erreichbarkeit; dies beweist bei einer Firewall keinen ausgeschalteten Zustand. |
| Zeitplan für die nächste Minute speichern | Im DSM-Aufgabenplaner existiert genau eine zugehörige SynoWake-Aufgabe. |
| Weckzeit ohne Namen oder nur mit Leerzeichen speichern | Name enthält Gerät, ausgewählte Tage und Uhrzeit; für alle Tage „Täglich“, für Mo–Fr diese Kurzform, sonst einzelne Tage. Derselbe Name steht mit SynoWake-Präfix im DSM-Aufgabenplaner. |
| Weckzeit mit eigenem Namen speichern | Der eingetragene Name wird übernommen. |
| SynoWake über DSM-Hauptmenü öffnen; Konto kann direkt im DSM Aufgaben anlegen | Aufgabenanlage verwendet die native DSM-Sitzung mit aktuellem Sitzungsschutz und wird nicht mit Fehler 105 abgelehnt. |
| Native Aufgabenanfrage verliert ihre Antwort | Kein direkter Ersatzaufruf und keine doppelte Anlage. Die Vorbereitung bleibt für den Aufgabenabgleich erhalten; eine verspätete Antwort registriert sie nicht nachträglich lokal. |
| API-Katalog meldet TaskScheduler-Version 3 | get/create/set beginnen mit Version 3 und wechseln ausschließlich bei Fehler 104 auf kompatible Alternativen. list verwendet 3, delete 2. Erfolgreiche Versionen werden pro Methode gemerkt. |
| DSM lehnt create-Version 4 mit Fehler 104 ab | Eine unterstützte Alternative wird verwendet. Genau eine Aufgabe wird angelegt; bei vollständiger Versionsablehnung bleibt ein Fehler mit den geprüften Versionen sichtbar. |
| Verbindung bei Aufgabenanlage unterbrochen | Keine automatische Wiederholung mit anderer Version. Die Vorbereitung bleibt für den Aufgabenabgleich erhalten. |
| Zeitplan ausführen | Testrechner startet; Anwendungsprotokoll enthält Zeit und Gerät. |
| Uhrzeit und Wochentage bearbeiten | Bestehende DSM-Aufgabe wird geändert; keine doppelte Aufgabe. |
| Zeitplan deaktivieren/aktivieren | DSM-Aufgabe und Oberflächenzustand stimmen überein. |
| Benachrichtigung aktivieren | DSM-Desktopmeldung mit Titel und tatsächlichem Ausführungstext erscheint bei der Ausführung; bei ausgeschaltetem Haken fehlt sie. |
| App-Fenster schließen und Zeitplan ausführen | Benachrichtigung erscheint weiterhin durch `preloadTexts`; Deutsch/Englisch und Gerätenamen mit Umlauten, Prozentzeichen oder spitzen Klammern prüfen. |
| Protokoll-Center öffnen | Die Wake-Aktion ist dort mit SynoWake-Kennung nachvollziehbar. Eine Diagnose in SynoWake darf nicht übergangen werden. |
| TCP-Empfänger als Ersatzweg einstellen | BSD/TCP-Empfang im Protokoll-Center und gespeicherter SynoWake-Port stimmen überein; Loopback-Versand erscheint im Protokoll-Center. |
| Empfang vorübergehend ausschalten | Neue Wake-Aktion bleibt im lokalen Verlauf als nicht übermittelt markiert; eine Diagnose ist sichtbar. |
| Empfang wieder einschalten und erneut senden | Vorgemerkter Eintrag wird übertragen; keine Wake-Aktion wird dadurch erneut ausgelöst. |
| Rückstau im Ausführungsprotokoll prüfen | Ausstehende Einträge werden beim Begrenzen älterer übertragener Einträge erhalten; bei 3000 ausstehenden Einträgen werden neue Wake-Aktionen vor dem Versand angehalten. |
| Paket stoppen | Ausführung eines vorhandenen Zeitplans sendet kein Magic Packet. |
| Paket wieder starten | Der Zeitplan kann wieder ausführen. |
| NAS neu starten | Paket und registrierte Aufgaben arbeiten nach dem Start weiter. |
| Paket aktualisieren | Geräte, Zeitpläne und Protokoll bleiben erhalten; Aufgaben werden nicht dupliziert. |
| Alle SynoWake-Zeitpläne entfernen | Zugehörige Aufgaben verschwinden auch aus dem DSM-Aufgabenplaner. |
| Paket anschließend deinstallieren | Kein SynoWake-Programmpfad und keine zugehörige Aufgabe bleiben zur Ausführung registriert. |

## Diagnose bei einem Fehler

Zusätzlich Paket-Zentrum und Widgets offen halten, SynoWake stoppen/starten und das App-Fenster öffnen/schließen. Texte, Zeilenhöhen, Symbole und Formularfelder außerhalb von SynoWake müssen unverändert bleiben. Nach dem Upgrade DSM einmal vollständig neu laden, damit bereits geladene Dateien der alten Version entfernt werden; beim beschriebenen Firefox-Ablauf mit Shift+Klick auf den Aktualisieren-Knopf. Geräte, Zeitpläne, bereits gesetzte Favoriten und Ausführungsprotokolle müssen beim Upgrade erhalten bleiben. Bei einem Upgrade von vor 0.1.5 beginnen vorhandene Geräte ohne bisheriges Favoritenfeld ohne Häkchen; gewünschte Kacheln einmal in der Geräteliste markieren.

Das sichtbare Fehlerbild, die DSM-Version, die Uhrzeit und die Aktion festhalten. Im SynoWake-Protokoll die Diagnose ansehen. Bei Problemen mit Installation oder Start zusätzlich `/var/log/packages/SynoWake.log` und die DSM-Paketbetriebsprotokolle prüfen. Die private Dateiablage in `/var/packages/SynoWake/var` enthält Zeitplan-Geheimnisse und gehört nicht in ungeschwärzte Supportberichte.

Bei einer Aufgabenplaner-WebAPI-Meldung den Fehlercode und die Methode erfassen. Den betreffenden Zeitplan auch direkt im DSM-Aufgabenplaner kontrollieren. Eine fehlgeschlagene Registrierung bleibt ein Fehler; den Zeitplan nicht als funktionierend abnehmen.

Wenn das native Fenster leer bleibt, zunächst die direkte UI-Adresse nach DSM-Anmeldung öffnen. Wenn die direkte Oberfläche funktioniert, liegt der Fehler bei der DSM-Fensterregistrierung. Wenn auch `api.cgi` fehlschlägt, Dienststatus, `/var/packages/SynoWake/var/service.log`, CGI-Ausführbarkeit und Authentifizierungsprüfung untersuchen. Die CGI-Weiterleitung muss den Socket erreichen, darf aber keine privaten Datendateien benötigen.

Wenn die Wake-Aktion im eigenen Verlauf erscheint, aber nicht im Protokoll-Center, die Diagnose für `synologset1` prüfen. Im Protokoll-Center zunächst ein Archivziel wählen und einen **BSD/TCP**-Empfänger erstellen. Dessen Port in SynoWake einstellen. Für den Loopback-Sender kein SSL aktivieren. Eine neue Aktion ausführen und die Log-Center-Anzeige prüfen. Nach Beheben des Empfängers vorgemerkte Einträge erneut senden. Ein erfolgreicher TCP-Versand bestätigt noch keinen Datenbankeintrag; diese Sichtprüfung ist Teil der Abnahme.

## Testergebnis erfassen

```text
NAS-Modell: DS918+
DSM-Version/Build:
NAS-Zeitzone:
SynoWake-Version:
SPK-SHA256:
Installation / CGI / Fenster:
Geräte / Suche / Wake / Status:
Zeitplan anlegen / editieren / deaktivieren / löschen:
Stop / Start / Neustart / Upgrade:
Protokoll-Center:
DSM-Benachrichtigungen:
Verbliebene Aufgaben nach Entfernen:
Offene Fehler:
```
