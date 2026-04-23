# requrse

Send HTTP requests until specific conditions are met.

## Features

- **Templated Requests**: Define URL, headers, and body using Go templates
- **Multiple Methods**: Support for GET, POST, and WebSocket connections
- **Stop Conditions**: Use jq expressions to define when to stop recursing
- **Wordlist Support**: Enumerate with list files (pitchfork mode)
- **Proxy Support**: Route requests through proxies
- **Authentication**: Token-based auth support
- **Output Control**: Save responses to files or print to stdout
- **Debug Mode**: Enable detailed logging
- **jq Filter**: Apply jq transformations to JSON output

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--template` | `-t` | Template YAML file to use |
| `--host` | `-H` | HTTP host (default: localhost) |
| `--auth` | `-a` | Authentication token |
| `--out` | `-o` | Output directory |
| `--ext` | `-e` | File extension (default: json) |
| `--extra` | `-e` | Extra data pairs (key=value) |
| `--list` | `-l` | List files for enumeration |
| `--mode` | `-m` | List mode (pitchfork) |
| `--proxy` | `-p` | Proxy to use |
| `--debug` | `-d` | Debug mode |
| `--jq` | `-j` | jq filter to apply to JSON output |


## Template Format

Templates are YAML files with the following structure:

```yaml
name: Description
url: http://{{ .Host }}/endpoint?param={{.Page}}
method: GET
headers:
  Authorization: Token {{ .AuthToken }}
  User-Agent: {{ .Extra.browser }}
stop_when:
  - 'select(.response.data | length > 100) | .'
```

### Available Context Variables

- `.Host` - Target host
- `.Page` - Current page number
- `.PageSize` - Items per page
- `.AuthToken` - Authentication token
- `.Extra` - Extra key-value pairs
- `.LastResponse.BodyObject` - Previous response as JSON object
- `.LastResponse.RawBody` - Previous raw response
- `.ListParams` - List values (0-indexed)

## Examples

### Paginated API Enumeration

```yaml
name: Paginated API
url: http://{{ .Host }}/api/users?page={{.Page}}&limit={{.PageSize}}
method: GET
headers:
  Authorization: Token {{ .AuthToken }}
stop_when:
  - 'select(.next == null) | .'
```

Run:
```bash
requrse -t paginated.yaml -H api.example.com -a $TOKEN -o results -ext json
```

### With jq Filter

Apply transformations to JSON output before displaying or saving:

```bash
requrse -t paginated.yaml -H api.example.com -j '.data.items | .[0]'
```

This extracts just the first item from the response. More complex filters:

```bash
requrse -t paginated.yaml -H api.example.com -j '[.data.items[] | select(.active == true)]'
```


### WebSocket Login Brute-Force

```yaml
name: WebSocket Login
url: ws://{{ .Host }}/ws
body: '{"username":"{{ index .ListParams 0 }}","password":"{{ index .ListParams 1 }}"}'
headers:
  Sec-WebSocket-Protocol: control
stop_when:
  - 'select(.body_object.success) | .'
```

Run:
```bash
requrse -t ws-login.yaml -H ws.example.com -l users.txt -l passwords.txt
```

### Proxy-Based Enumeration

```yaml
name: Reverse Proxy Target
url: http://{{ .Host }}/{{ .Extra.target_path }}
method: GET
stop_when:
  - 'select(.status == 404) | .'
```

Run:
```bash
requrse -t proxy.yaml -H localhost -p http://10.0.0.1:8080 -e target_path=/admin
```

## License

MIT - see LICENSE file for details.
