package server

import (
	"html/template"
	"log/slog"
	"net/http"
)

// funcMap defines template helper functions
var funcMap = template.FuncMap{
	"ge": func(a, b float64) bool {
		return a >= b
	},
}

// renderStatsHTML renders the stats page using the HTML template
func renderStatsHTML(w http.ResponseWriter, stats map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")

	tmpl, err := template.New("stats").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		slog.Error("Error parsing HTML template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, stats); err != nil {
		slog.Error("Error executing HTML template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="refresh" content="30">
    <title>Monic Status</title>
    <style>
        :root {
            --bg-color: #1a1b26;
            --card-bg: #24283b;
            --text-color: #c0caf5;
            --accent: #7aa2f7;
            --success: #9ece6a;
            --warning: #e0af68;
            --danger: #f7768e;
            --border: #414868;
        }
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-color);
            margin: 0;
            padding: 20px;
            line-height: 1.6;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 30px;
            border-bottom: 1px solid var(--border);
            padding-bottom: 20px;
        }
        h1 { margin: 0; color: var(--accent); }
        .status-badge {
            background-color: var(--success);
            color: #1a1b26;
            padding: 5px 10px;
            border-radius: 4px;
            font-weight: bold;
            text-transform: uppercase;
            font-size: 0.8em;
        }
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        .card {
            background-color: var(--card-bg);
            border-radius: 8px;
            padding: 20px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        h2 { margin-top: 0; border-bottom: 1px solid var(--border); padding-bottom: 10px; font-size: 1.2em; }
        .stat-row {
            display: flex;
            justify-content: space-between;
            margin-bottom: 10px;
        }
        .stat-label { color: #787c99; }
        .stat-value { font-weight: bold; }
        .progress-bar {
            background-color: var(--border);
            height: 8px;
            border-radius: 4px;
            overflow: hidden;
            margin-top: 5px;
        }
        .progress-fill {
            height: 100%;
            background-color: var(--accent);
            transition: width 0.3s ease;
        }
        .status-ok { color: var(--success); }
        .status-fail { color: var(--danger); }
        table {
            width: 100%;
            border-collapse: collapse;
        }
        th, td {
            text-align: left;
            padding: 10px;
            border-bottom: 1px solid var(--border);
        }
        th { color: #787c99; }
        .alert-item {
            padding: 10px;
            border-left: 4px solid var(--accent);
            background-color: rgba(0,0,0,0.2);
            margin-bottom: 10px;
        }
        .alert-critical { border-left-color: var(--danger); }
        .alert-warning { border-left-color: var(--warning); }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <div>
                <h1>Monic Status</h1>
                <small>Uptime: {{.service_status.uptime}}</small>
            </div>
            <div class="status-badge">{{.service_status.status}}</div>
        </header>

        <div class="grid">
            <!-- System Info -->
            <div class="card">
                <h2>System Resources</h2>
                {{if .current_system_stats}}
                <div class="stat-group">
                    <div class="stat-row">
                        <span class="stat-label">CPU Usage</span>
                        <span class="stat-value">{{printf "%.1f" .current_system_stats.cpu_usage}}%</span>
                    </div>
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: {{.current_system_stats.cpu_usage}}%; background-color: {{if ge .current_system_stats.cpu_usage 80.0}}var(--danger){{else}}var(--accent){{end}}"></div>
                    </div>
                </div>
                <br>
                <div class="stat-group">
                    <div class="stat-row">
                        <span class="stat-label">Memory Usage</span>
                        <span class="stat-value">{{printf "%.1f" .current_system_stats.memory_usage.used_percent}}%</span>
                    </div>
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: {{.current_system_stats.memory_usage.used_percent}}%; background-color: {{if ge .current_system_stats.memory_usage.used_percent 85.0}}var(--danger){{else}}var(--accent){{end}}"></div>
                    </div>
                </div>
                {{else}}
                <p>No system stats available</p>
                {{end}}
            </div>

            <!-- System Details -->
            <div class="card">
                <h2>System Details</h2>
                <div class="stat-row">
                    <span class="stat-label">Hostname</span>
                    <span class="stat-value">{{.system_info.hostname}}</span>
                </div>
                <div class="stat-row">
                    <span class="stat-label">Platform</span>
                    <span class="stat-value">{{.system_info.platform}}</span>
                </div>
                <div class="stat-row">
                    <span class="stat-label">Arch</span>
                    <span class="stat-value">{{.system_info.arch}}</span>
                </div>
                <div class="stat-row">
                    <span class="stat-label">Active Alerts</span>
                    <span class="stat-value">{{.alerts.active_alerts}}</span>
                </div>
            </div>
        </div>

        <!-- HTTP Checks -->
        <div class="card">
            <h2>HTTP Checks</h2>
            <table>
                <thead>
                    <tr>
                        <th>Name</th>
                        <th>URL</th>
                        <th>Status</th>
                        <th>Response Time</th>
                        <th>Last Check</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .http_checks}}
                    <tr>
                        <td>{{.name}}</td>
                        <td><a href="{{.url}}" target="_blank" style="color: var(--accent)">{{.url}}</a></td>
                        <td>
                            {{if eq .status "success"}}
                            <span class="status-ok">● Online</span>
                            {{else}}
                            <span class="status-fail">● Offline</span>
                            {{end}}
                        </td>
                        <td>{{.response_time}}</td>
                        <td>{{.last_check}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>

        <br>

        <!-- Recent Alerts -->
        {{if .alerts.recent_alerts}}
        <div class="card">
            <h2>Recent Alerts</h2>
            {{range .alerts.recent_alerts}}
            <div class="alert-item alert-{{.level}}">
                <div class="stat-row">
                    <strong>{{.type}}</strong>
                    <small>{{.timestamp}}</small>
                </div>
                <div>{{.message}}</div>
            </div>
            {{end}}
        </div>
        {{end}}
    </div>
</body>
</html>
`
