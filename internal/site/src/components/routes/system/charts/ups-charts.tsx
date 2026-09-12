import { t } from "@lingui/core/macro"
import { useLingui } from "@lingui/react/macro"
import { Fragment, useState } from "react"
import LineChartDefault from "@/components/charts/line-chart"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { cn, decimalString } from "@/lib/utils"
import type { ChartData, SystemRecord, SystemStatsRecord, UPSStats } from "@/types"
import { ChartCard } from "../chart-card"

function statusLabel(flag: string) {
	switch (flag) {
		case "OL": return t`Utility power`
		case "OB": return t`On battery`
		case "LB": return t`Low battery`
		case "RB": return t`Replace battery`
		case "OVER": return t`Overload`
		case "CHRG": return t`Charging`
		case "DISCHRG": return t`Discharging`
		case "BYPASS": return t`Bypass`
		case "OFF": return t`Output off`
		case "FSD": return t`Forced shutdown`
		case "ALARM": return t`UPS alarm`
		default: return flag
	}
}

export function UPSCharts({ chartData, grid, dataEmpty, maxValues, system }: {
	chartData: ChartData
	grid: boolean
	dataEmpty: boolean
	maxValues: boolean
	system: SystemRecord
}) {
	const { t } = useLingui()
	const [selected, setSelected] = useState("")
	const ids = new Set(Object.keys(system.info.ups ?? {}))
	const names = [...ids].sort()
	const id = ids.has(selected) ? selected : names[0]
	if (!id) return null
	const current = system.info.ups?.[id]
	// Current readings always come from live system info, independently of the chart range.
	const fresh = system.status === "up" && current?.online && Date.now() / 1000 - current.updated < 120
	const flags = fresh ? current.status?.split(/\s+/).filter(Boolean) ?? [] : []
	const warning = !fresh || flags.some((flag) => ["OB", "LB", "RB", "OVER", "FSD", "ALARM", "OFF"].includes(flag))
	const groups = [
		{ title: t`UPS charge and load`, unit: "%", keys: ["battery.charge", "ups.load"], labels: [t`Battery charge`, t`UPS load`], scale: 1 },
		{ title: t`UPS runtime`, unit: t`min`, keys: ["battery.runtime"], labels: [t`Estimated runtime`], scale: 60 },
		{ title: t`UPS power`, unit: "W", keys: ["ups.realpower"], labels: [t`Power`], scale: 1 },
		{ title: t`UPS voltage`, unit: "V", keys: ["input.voltage", "output.voltage", "battery.voltage"], labels: [t`Input voltage`, t`Output voltage`, t`Battery voltage`], scale: 1 },
		{ title: t`Output frequency`, unit: "Hz", keys: ["output.frequency"], labels: [t`Output frequency`], scale: 1 },
		{ title: t`UPS temperature`, unit: "°C", keys: ["ups.temperature", "battery.temperature"], labels: [t`UPS temperature`, t`Battery temperature`], scale: 1 },
	]
	const detailValue = (key: string, value: string) => {
		if (key === "ups.beeper.status") {
			switch (value) {
				case "enabled": return t`Enabled`
				case "disabled": return t`Disabled`
				case "muted": return t`Muted`
			}
		}
		if (key === "ups.type") {
			switch (value) {
				case "offline / line interactive": return t`Offline / line interactive`
				case "offline": return t`Offline UPS`
				case "line interactive": return t`Line interactive UPS`
				case "online": return t`Online UPS`
			}
		}
		return value
	}
	return (
		<Fragment>
			<Card className="col-span-full">
				<CardHeader>
					<div className="flex flex-wrap items-center justify-between gap-3">
						<CardTitle>{id.toLowerCase() === "ups" ? "UPS" : `UPS · ${id}`}</CardTitle>
						{names.length > 1 && <Select value={id} onValueChange={setSelected}>
							<SelectTrigger className="w-48" aria-label={t`Select UPS`}><SelectValue /></SelectTrigger>
							<SelectContent>{names.map((name) => <SelectItem key={name} value={name}>{name}</SelectItem>)}</SelectContent>
						</Select>}
					</div>
					{current?.model && current.model !== id && <CardDescription>{current.model}</CardDescription>}
					<p className={cn("text-sm font-medium", warning ? "text-destructive" : "text-emerald-600 dark:text-emerald-400")}>
						{t`Status`}: {fresh ? flags.map((flag) => {
							const label = statusLabel(flag)
							return label === flag ? flag : `${label} (${flag})`
						}).join(" · ") || t`Unknown` : t`UPS unavailable or data stale`}
					</p>
					<CardDescription>{t`Last successful reading`}: {current?.updated ? new Date(current.updated * 1000).toLocaleString() : "—"}</CardDescription>
				</CardHeader>
				<CardContent>
					{current?.details && <dl className="mt-6 grid grid-cols-2 gap-4 border-t pt-4 sm:grid-cols-4">
						{[
							["ups.type", t`UPS type`, ""],
							["ups.beeper.status", t`Beeper status`, ""],
							["battery.voltage.nominal", t`Nominal battery voltage`, "V"],
							["battery.voltage.high", t`Battery high voltage reference`, "V"],
							["battery.voltage.low", t`Battery low voltage reference`, "V"],
							["output.voltage.nominal", t`Nominal output voltage`, "V"],
							["output.current.nominal", t`Nominal output current`, "A"],
							["output.frequency.nominal", t`Nominal output frequency`, "Hz"],
							["ups.delay.shutdown", t`Shutdown delay`, "s"],
							["ups.delay.start", t`Startup delay`, "s"],
						].filter(([key]) => current.details?.[key] !== undefined).map(([key, label, unit]) => (
							<div key={key}><dt className="text-sm text-muted-foreground">{label}</dt><dd className="mt-1 break-words">{fresh ? `${detailValue(key, current.details?.[key] ?? "")} ${unit}` : "—"}</dd></div>
						))}
					</dl>}
				</CardContent>
			</Card>
			{groups.map((group) => {
				const keys = group.keys.filter((key) => current?.metrics?.[key] !== undefined || chartData.systemStats.some((record) => record.stats?.ups?.[id]?.metrics?.[key] !== undefined))
				if (!keys.length) return null
				return <ChartCard key={`${id}:${group.title}`} empty={dataEmpty || !chartData.systemStats.some((record) => keys.some((key) => record.stats?.ups?.[id]?.metrics?.[key] !== undefined))} grid={grid} legend={keys.length > 1} title={group.title} description={id}>
					<LineChartDefault
						key={`${id}:${maxValues}`}
						chartData={chartData}
						maxToggled={maxValues}
						legend={keys.length > 1}
						dataPoints={keys.map((key, index) => ({
							label: group.labels[group.keys.indexOf(key)],
							color: index + 1,
							dataKey: ({ stats }: SystemStatsRecord) => {
								const sample: UPSStats | undefined = stats?.ups?.[id]
								const value = (maxValues ? sample?.max?.[key] : undefined) ?? sample?.metrics?.[key]
								return value === undefined ? undefined : value / group.scale
							},
						}))}
						tickFormatter={(value) => `${decimalString(value, 1)} ${group.unit}`}
						contentFormatter={({ value }) => `${decimalString(value, 1)} ${group.unit}`}
					/>
				</ChartCard>
			})}
		</Fragment>
	)
}
