package transmission

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/meinside/telegram-bot-remotecontrol/conf"
	"github.com/meinside/telegram-bot-remotecontrol/helper"
)

const (
	httpHeaderXTransmissionSessionId = "X-Transmission-Session-Id"
	numRetries                       = 3
)

type transmissionRpcRequest struct {
	Method    string                 `json:"method"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Tag       int                    `json:"tag,omitempty"`
}

type transmissionRpcResponse struct {
	Result    string                      `json:"result,omitempty"`
	Arguments transmissionRpcResponseArgs `json:"arguments,omitempty"`
	Tag       int                         `json:"tag,omitempty"`
}

type transmissionRpcResponseArgs struct {
	TorrentDuplicate interface{}                      `json:"torrent-duplicate,omitempty"`
	Torrents         []transmissionRpcResponseTorrent `json:"torrents,omitempty"`
}

var torrentFields []string = []string{
	"id",
	"name",
	"percentDone",
	"totalSize",
	"errorString",
}

type transmissionRpcResponseTorrent struct {
	Id          int     `json:"id"`
	Name        string  `json:"name"`
	PercentDone float32 `json:"percentDone"`
	TotalSize   int64   `json:"totalSize"`
	Error       string  `json:"errorString"`
}

var xTransmissionSessionId string = ""

func getLocalTransmissionRpcUrl(port int, username, passwd string) string {
	var rpcUrl string
	if len(username) > 0 && len(passwd) > 0 {
		rpcUrl = fmt.Sprintf("http://%s:%s@localhost:%d/transmission/rpc", url.QueryEscape(username), url.QueryEscape(passwd), port)
	} else {
		rpcUrl = fmt.Sprintf("http://localhost:%d/transmission/rpc", port)
	}
	return rpcUrl
}

// POST to Transmission RPC server
//
// https://trac.transmissionbt.com/browser/trunk/extras/rpc-spec.txt
func post(port int, username, passwd string, request transmissionRpcRequest, numRetriesLeft int) (res []byte, err error) {
	if numRetriesLeft <= 0 {
		return res, fmt.Errorf("No more retries for this request: %v", request)
	}

	var data []byte
	if data, err = json.Marshal(request); err == nil {
		var req *http.Request
		if req, err = http.NewRequest("POST", getLocalTransmissionRpcUrl(port, username, passwd), bytes.NewBuffer(data)); err == nil {
			// headers
			req.Header.Set(httpHeaderXTransmissionSessionId, xTransmissionSessionId)

			var resp *http.Response
			client := &http.Client{}
			if resp, err = client.Do(req); err == nil {
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusConflict {
					if sessionId, exists := resp.Header[httpHeaderXTransmissionSessionId]; exists && len(sessionId) > 0 {
						// update session id
						xTransmissionSessionId = sessionId[0]

						return post(port, username, passwd, request, numRetriesLeft-1) // XXX - retry
					} else {
						err = fmt.Errorf("Could not find '%s' value from http headers", httpHeaderXTransmissionSessionId)

						log.Printf("Error in RPC server: %s\n", err.Error())
					}
				}

				res, _ = ioutil.ReadAll(resp.Body)

				if resp.StatusCode != http.StatusOK {
					err = fmt.Errorf("HTTP %d (%s)", resp.StatusCode, string(res))

					log.Printf("Error from RPC server: %s\n", err.Error())
				} /* else {
					// XXX - for debugging
					log.Printf("returned json = %s\n", string(res))
				}*/
			} else {
				log.Printf("Error while sending request: %s\n", err.Error())

				return post(port, username, passwd, request, numRetriesLeft-1) // XXX - retry
			}
		} else {
			log.Printf("Error while building request: %s\n", err.Error())
		}
	} else {
		log.Printf("Error while marshaling data: %s\n", err.Error())
	}

	return res, err
}

// for retrieving torrent objects
func GetTorrents(port int, username, passwd string) (torrents []transmissionRpcResponseTorrent, err error) {
	var output []byte
	if output, err = post(port, username, passwd,
		transmissionRpcRequest{
			Method: "torrent-get",
			Arguments: map[string]interface{}{
				"fields": torrentFields,
			},
		}, numRetries); err == nil {
		var result transmissionRpcResponse
		if err = json.Unmarshal(output, &result); err == nil {
			if result.Result == "success" {
				torrents = result.Arguments.Torrents
			} else {
				err = fmt.Errorf("Failed to list torrents.")
			}
		}
	}
	return torrents, err
}

// for showing the list of transmission
func GetList(port int, username, passwd string) string {
	if torrents, err := GetTorrents(port, username, passwd); err == nil {
		numTorrents := len(torrents)
		if numTorrents > 0 {
			strs := make([]string, numTorrents)
			for i, t := range torrents {
				if len(t.Error) > 0 {
					strs[i] = fmt.Sprintf("%d. _%s_ (total %s, *%s*)", t.Id, helper.RemoveMarkdownChars(t.Name, " "), readableSize(t.TotalSize), t.Error)
				} else {
					strs[i] = fmt.Sprintf("%d. _%s_ (total %s, *%.2f%%*)", t.Id, helper.RemoveMarkdownChars(t.Name, " "), readableSize(t.TotalSize), t.PercentDone*100.0)
				}
			}
			return strings.Join(strs, "\n")
		} else {
			return conf.MessageTransmissionNoTorrents
		}
	} else {
		return err.Error()
	}
}

// for adding a torrent(with magnet or .torrent file) to the list of transmission
func AddTorrent(port int, username, passwd, torrent string) string {
	if output, err := post(port, username, passwd,
		transmissionRpcRequest{
			Method: "torrent-add",
			Arguments: map[string]interface{}{
				"filename": torrent,
			},
		}, numRetries); err == nil {
		var result transmissionRpcResponse
		if err := json.Unmarshal(output, &result); err == nil {
			if result.Result == "success" {
				if result.Arguments.TorrentDuplicate != nil {
					return "Duplicated torrent was given."
				} else {
					return "Given torrent was successfully added to the list."
				}
			} else {
				return fmt.Sprintf("Failed to add given torrent")
			}
		} else {
			return fmt.Sprintf("Malformed RPC server response: %s", string(output))
		}
	} else {
		return fmt.Sprintf("Failed to add given torrent - %s", string(output))
	}
}

func removeTorrent(port int, username, passwd, torrentId string, deleteLocal bool) string {
	if numId, err := strconv.Atoi(torrentId); err == nil {
		if output, err := post(port, username, passwd,
			transmissionRpcRequest{
				Method: "torrent-remove",
				Arguments: map[string]interface{}{
					"ids":               []int{numId},
					"delete-local-data": deleteLocal,
				},
			}, numRetries); err == nil {
			var result transmissionRpcResponse
			if err := json.Unmarshal(output, &result); err == nil {
				if result.Result == "success" {
					if deleteLocal {
						return fmt.Sprintf("Torrent id %s and its data were successfully deleted.", torrentId)
					} else {
						return fmt.Sprintf("Torrent id %s was successfully removed from the list.", torrentId)
					}
				} else {
					return fmt.Sprintf("Failed to remove given torrent")
				}
			} else {
				return fmt.Sprintf("Malformed RPC server response: %s", string(output))
			}
		} else {
			return fmt.Sprintf("Failed to remove given torrent - %s", err.Error())
		}
	} else {
		return fmt.Sprintf("Not a valid torrent id: ", torrentId)
	}
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

// for canceling/removing a torrent from the list
func RemoveTorrent(port int, username, passwd, torrentId string) string {
	return removeTorrent(port, username, passwd, torrentId, false)
}

// for removing a torrent and its local data from the list
func DeleteTorrent(port int, username, passwd, torrentId string) string {
	return removeTorrent(port, username, passwd, torrentId, true)
}
