# UPS monitoring with NUT

Beszel can read one or more UPS devices from a Network UPS Tools (NUT) `upsd`
server. Run NUT on the machine connected to the UPS (or use an existing NUT
service on your NAS). Beszel reads the NUT TCP protocol directly: the agent does
not need `upsc`, USB access, or privileged container mode.

Both the hub and the collecting agent must be built from this version. The hub
migration adds UPS alert types automatically on startup. Existing agents and
systems without UPS configuration continue to work.

## Agent configuration

Add these environment variables to the **agent**, then restart it:

```yaml
environment:
  NUT_HOST: "192.168.1.10:3493"
  NUT_UPS: "ups"
```

Keep the agent's existing connection settings. `NUT_UPS` is the device name in
the NUT server's `ups.conf`, not the model name. Use comma-separated names for
multiple devices, for example `ups,backup`. A maximum of 32 devices on one NUT
server is supported. The port defaults to 3493; IPv6 with an explicit port uses
the form `[::1]:3493`.

Optional `NUT_USERNAME` and `NUT_PASSWORD` configure authentication if required
by the server. The client only authenticates and reads `LIST VAR`;
it never requests shutdown, changes settings, or takes the UPS monitor role.
This implementation uses plain TCP, so use a trusted LAN or protected tunnel;
servers requiring NUT STARTTLS are not supported yet.

For a containerized agent, `localhost` means the container itself unless using
host networking. Use a hostname or address reachable from the agent. If one UPS
powers several servers, configure collection on one agent to avoid duplicate
records and notifications.

To find device names from a machine with NUT tools installed:

```sh
upsc -l 192.168.1.10
upsc ups@192.168.1.10
```

## Data and charts

The agent polls every 10 seconds in the background, independently of hub
requests. Each poll has a five-second total deadline, including a two-second
connection timeout. Failed devices report communication loss, keep their last
successful timestamp, and omit measurements. Snapshots older than 30 seconds
are marked stale by the agent. Credentials are never included in metric data.

The system detail page contains a UPS selector, live status and readings, and
historical charts. The current card reads live system info even while viewing
longer historical ranges. Historical series follow Beszel's existing reporting
and retention intervals; a 10-second agent poll does not imply 10-second history
or notifications. Short outages between hub samples may not appear in history.

| NUT variable | Unit |
| --- | --- |
| `battery.charge` | % |
| `battery.runtime` | seconds (displayed as minutes) |
| `ups.load` | % |
| `ups.realpower` | W |
| `input.voltage`, `output.voltage`, `battery.voltage` | V |
| `ups.temperature`, `battery.temperature` | °C |

Only available measurements are shown. Zero is a valid reading; missing,
negative non-temperature readings and non-finite values are omitted. Real power
is not estimated from VA ratings or load percentage. Model and NUT status flags
are also retained. `online` in the payload means **communication available**, not
utility power: `OB` means battery supply and `OL` means utility supply.

Historical rollups keep weighted means, minima, maxima, and observed status
flags per device. Missing values are not counted as zero. The existing maximum
toggle shows peak measurements. Raw communication gaps are not connected by
chart lines. This version does not add a separate outage event log or automatic
shutdown controls.

## Alerts

Enable alerts from the system's existing alert settings:

- UPS battery charge: below the configured percentage (default 20%).
- UPS runtime: below the configured number of minutes (default 5).
- UPS load: above the configured percentage (default 80%).
- UPS on battery: any UPS reports `OB`.
- UPS communication: any configured UPS is unreachable or stale.
- UPS fault: any UPS reports `LB`, `OVER`, `RB`, `FSD`, or `ALARM`.

Rules are per system and cover all configured UPS devices: minimum charge or
runtime, maximum load, and any matching status flag. Numeric rules use the
existing configurable averaging window. Binary rules trigger on the next hub
observation, with no configurable delay. Recovery notifications use the
existing alert mechanism. Unknown data never counts as a healthy recovery;
known anomalies can still trigger if another device's reading is missing.

Low charge/runtime rules also apply while utility power is present. Per-device
alert routing and sub-minute outage event capture are not part of this version.

Protocol reference: [NUT network protocol](https://networkupstools.org/docs/developer-guide.chunked/net-protocol.html).
