package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/meinside/telegram-remotecontrol-bot/consts"
)

const (
	httpHeaderXTransmissionSessionID = `X-Transmission-Session-Id`
	numRetries                       = 3
)

// RPC request
type rpcRequest struct {
	Method    string         `json:"method"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Tag       int            `json:"tag,omitempty"`
}

// RPC response
type rpcResponse struct {
	Result    string          `json:"result,omitempty"`
	Arguments rpcResponseArgs `json:"arguments,omitzero"`
	Tag       int             `json:"tag,omitempty"`
}

// RPC response arguments
type rpcResponseArgs struct {
	TorrentDuplicate any                  `json:"torrent-duplicate,omitempty"`
	Torrents         []RPCResponseTorrent `json:"torrents,omitempty"`
}

// torrent fields to query
var torrentFields []string = []string{
	"id",
	"status",
	"name",
	"rateDownload", // B/s
	"rateUpload",   // B/s
	"percentDone",
	"totalSize",
	"errorString",
}

// RPCResponseTorrent for torrent response
type RPCResponseTorrent struct {
	ID           int           `json:"id"`
	Status       TorrentStatus `json:"status"`
	Name         string        `json:"name"`
	RateDownload int64         `json:"rateDownload"`
	RateUpload   int64         `json:"rateUpload"`
	PercentDone  float32       `json:"percentDone"`
	TotalSize    int64         `json:"totalSize"`
	Error        string        `json:"errorString"`
}

type TorrentStatus int

const (
	TorrentStatusStopped                 TorrentStatus = 0
	TorrentStatusQueuedToVerifyLocalData TorrentStatus = 1
	TorrentStatusVerifyingLocalData      TorrentStatus = 2
	TorrentStatusQueuedToDownload        TorrentStatus = 3
	TorrentStatusDownloading             TorrentStatus = 4
	TorrentStatusQueuedToSeed            TorrentStatus = 5
	TorrentStatusSeeding                 TorrentStatus = 6
)

var xTransmissionSessionID string = ""

// convert torrent status to string
func statusToString(s TorrentStatus) string {
	switch s {
	case TorrentStatusStopped:
		return `⛔` // Stopped
	case TorrentStatusQueuedToVerifyLocalData:
		return `⏳🔍` // Queued to verify local data
	case TorrentStatusVerifyingLocalData:
		return `🔍` // Verifying local data
	case TorrentStatusQueuedToDownload:
		return `⏳📥` // Queued to download
	case TorrentStatusDownloading:
		return `📥` // Downloading
	case TorrentStatusQueuedToSeed:
		return `⏳🌱` // Queued to seed
	case TorrentStatusSeeding:
		return `🌱` // Seeding
	default:
		return `❓` // Unknown
	}
}

// generate a RPC url for local transmission server
func getLocalTransmissionRPCURL(
	port int,
	username, passwd string,
) string {
	var rpcURL string
	if len(username) > 0 && len(passwd) > 0 {
		rpcURL = fmt.Sprintf("http://%s:%s@localhost:%d/transmission/rpc", url.QueryEscape(username), url.QueryEscape(passwd), port)
	} else {
		rpcURL = fmt.Sprintf("http://localhost:%d/transmission/rpc", port)
	}
	return rpcURL
}

// POST to Transmission RPC server
//
// https://trac.transmissionbt.com/browser/trunk/extras/rpc-spec.txt
func post(
	port int,
	username, passwd string,
	request rpcRequest,
	numRetriesLeft int,
) (res []byte, err error) {
	if numRetriesLeft <= 0 {
		return res, fmt.Errorf("no more retries for this request: %v", request)
	}

	var data []byte
	if data, err = json.Marshal(request); err == nil {
		var req *http.Request
		if req, err = http.NewRequest("POST", getLocalTransmissionRPCURL(port, username, passwd), bytes.NewBuffer(data)); err == nil {
			// headers
			req.Header.Set(httpHeaderXTransmissionSessionID, xTransmissionSessionID)

			var resp *http.Response
			client := &http.Client{}
			if resp, err = client.Do(req); err == nil {
				defer func() { _ = resp.Body.Close() }()

				if resp.StatusCode == http.StatusConflict {
					if sessionID, exists := resp.Header[httpHeaderXTransmissionSessionID]; exists && len(sessionID) > 0 {
						// update session id
						xTransmissionSessionID = sessionID[0]

						return post(port, username, passwd, request, numRetriesLeft-1) // XXX - retry
					}

					err = fmt.Errorf("couldn't find '%s' value from http headers", httpHeaderXTransmissionSessionID)

					log.Printf("error in RPC server: %s\n", err.Error())
				}

				res, _ = io.ReadAll(resp.Body)

				if resp.StatusCode != http.StatusOK {
					err = fmt.Errorf("HTTP %d (%s)", resp.StatusCode, string(res))

					log.Printf("error from RPC server: %s\n", err.Error())
				} /* else {
					// XXX - for debugging
					log.Printf("returned json = %s\n", string(res))
				}*/
			} else {
				log.Printf("error while sending request: %s\n", err.Error())

				return post(port, username, passwd, request, numRetriesLeft-1) // XXX - retry
			}
		} else {
			log.Printf("error while building request: %s\n", err.Error())
		}
	} else {
		log.Printf("error while marshaling data: %s\n", err.Error())
	}

	return res, err
}

// GetTorrents retrieves torrent objects.
func GetTorrents(
	port int,
	username, passwd string,
) (torrents []RPCResponseTorrent, err error) {
	var output []byte
	if output, err = post(
		port,
		username,
		passwd,
		rpcRequest{
			Method: "torrent-get",
			Arguments: map[string]any{
				"fields": torrentFields,
			},
		},
		numRetries,
	); err == nil {
		var result rpcResponse
		if err = json.Unmarshal(output, &result); err == nil {
			if result.Result == "success" {
				torrents = result.Arguments.Torrents
			} else {
				err = fmt.Errorf("failed to list torrents")
			}
		}
	}
	return torrents, err
}

// GetList retrieves the list of transmission.
func GetList(
	port int,
	username, passwd string,
) string {
	var torrents []RPCResponseTorrent
	var err error
	if torrents, err = GetTorrents(port, username, passwd); err == nil {
		numTorrents := len(torrents)
		if numTorrents > 0 {
			lines := []string{}

			for _, t := range torrents {
				if len(t.Error) > 0 {
					lines = append(lines, fmt.Sprintf(
						`*%d*. _%s_
  ┖ (%s) *%s*`,
						t.ID,
						removeMarkdownChars(t.Name, " "),
						readableSize(t.TotalSize),
						t.Error,
					))
				} else {
					details := []string{}

					switch t.Status {
					case TorrentStatusSeeding:
						details = append(
							details,
							fmt.Sprintf(
								"%s %s",
								statusToString(t.Status),
								readableSize(t.TotalSize),
							),
						)
						if t.RateUpload > 0 {
							details = append(details, fmt.Sprintf("↑%s/s", readableSize(t.RateUpload)))
						}
					case TorrentStatusDownloading, TorrentStatusStopped:
						details = append(
							details,
							fmt.Sprintf(
								"%s %s/%s (%.2f%%)",
								statusToString(t.Status),
								readableSize(int64(float64(t.TotalSize)*float64(t.PercentDone))),
								readableSize(t.TotalSize),
								t.PercentDone*100.0,
							),
						)
						updown := []string{}
						if t.RateDownload > 0 {
							updown = append(updown, fmt.Sprintf("↓%s/s", readableSize(t.RateDownload)))
						}
						if t.RateUpload > 0 {
							updown = append(updown, fmt.Sprintf("↑%s/s", readableSize(t.RateUpload)))
						}
						if len(updown) > 0 {
							details = append(details, strings.Join(updown, " "))
						}
					default:
						details = append(
							details,
							statusToString(t.Status),
						)
					}
					// prepend spaces to details
					for i := range details {
						details[i] = `  ┖ ` + details[i]
					}

					lines = append(lines, fmt.Sprintf(
						`*%d*. _%s_
%s`,
						t.ID,
						removeMarkdownChars(t.Name, " "),
						strings.Join(details, "\n"),
					))
				}
			}
			lines = append(lines, `----`)
			lines = append(lines, fmt.Sprintf("total %d torrent(s)", numTorrents))

			return strings.Join(lines, "\n")
		}

		return consts.MessageTransmissionNoTorrents
	}

	return err.Error()
}

// AddTorrent adds a torrent(with magnet or .torrent file) to the list of transmission
// and returns the resulting string.
func AddTorrent(port int, username, passwd, torrent string) string {
	var output []byte
	var err error
	if output, err = post(port, username, passwd, rpcRequest{
		Method: "torrent-add",
		Arguments: map[string]any{
			"filename": torrent,
		},
	}, numRetries); err == nil {
		var result rpcResponse
		if err = json.Unmarshal(output, &result); err == nil {
			if result.Result == "success" {
				if result.Arguments.TorrentDuplicate != nil {
					return "Duplicated torrent was given."
				}

				return "Given torrent was successfully added to the list."
			}

			return "Failed to add given torrent."
		}

		return fmt.Sprintf("Malformed RPC server response: %s", string(output))
	}

	return fmt.Sprintf("Failed to add given torrent: %s", err)
}

// remove torrent
func removeTorrent(
	port int,
	username, passwd string,
	torrentID string, deleteLocal bool,
) string {
	if numID, err := strconv.Atoi(torrentID); err == nil {
		if output, err := post(port, username, passwd,
			rpcRequest{
				Method: "torrent-remove",
				Arguments: map[string]any{
					"ids":               []int{numID},
					"delete-local-data": deleteLocal,
				},
			}, numRetries); err == nil {
			var result rpcResponse
			if err := json.Unmarshal(output, &result); err == nil {
				if result.Result == "success" {
					if deleteLocal {
						return fmt.Sprintf("Torrent id: %s and its data were successfully deleted", torrentID)
					}

					return fmt.Sprintf("Torrent id: %s was successfully removed from the list", torrentID)
				}

				return "Failed to remove given torrent."
			}

			return fmt.Sprintf("Malformed RPC server response: %s", string(output))
		}

		return fmt.Sprintf("Failed to remove given torrent: %s", err)
	}

	return fmt.Sprintf("not a valid torrent id: %s", torrentID)
}

// convert given number to human-readable size string
func readableSize(num int64) (str string) {
	if num < 1<<10 {
		// bytes
		str = fmt.Sprintf("%dB", num)
	} else {
		if num < 1<<20 {
			// kbytes
			str = fmt.Sprintf("%.1fKB", float64(num)/(1<<10))
		} else {
			if num < 1<<30 {
				// mbytes
				str = fmt.Sprintf("%.1fMB", float64(num)/(1<<20))
			} else {
				if num < 1<<40 {
					// gbytes
					str = fmt.Sprintf("%.2fGB", float64(num)/(1<<30))
				} else {
					// tbytes
					str = fmt.Sprintf("%.2fTB", float64(num)/(1<<40))
				}
			}
		}
	}
	return str
}

// RemoveTorrent cancels/removes a torrent from the list.
func RemoveTorrent(
	port int,
	username, passwd string,
	torrentID string,
) string {
	return removeTorrent(port, username, passwd, torrentID, false)
}

// DeleteTorrent removes a torrent and its local data from the list.
func DeleteTorrent(
	port int,
	username, passwd string,
	torrentID string,
) string {
	return removeTorrent(port, username, passwd, torrentID, true)
}
