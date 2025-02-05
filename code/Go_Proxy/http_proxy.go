package main

import (
	"bufio"
    "crypto/rsa"
    "encoding/base64"
    "fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const MAX_BUFFER = 4 * 1460
const (
	SERVERROR = 500
	BADREQ    = 400
)

func sendError(w net.Conn, err int) {
	// send error message correspding to error code err
	switch err {
	case SERVERROR:
		io.WriteString(w, "500 Internal Server Error\r\n")
	case BADREQ:
		io.WriteString(w, "400 Bad Request\r\n")
	default:
		io.WriteString(w, "999 Programmer Error\r\n")
	}
}

func getExitProxy(url string) string {
	// read exit proxy table to fill in IP:Port
    file, err := os.Open("url_exit_mapping.txt")
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()
    scanner := bufio.NewScanner(file)
    var exit_address string
    for scanner.Scan() {
        s := strings.Split(scanner.Text(), ",")
        url_from_map, exit := s[0], s[1]
        if url == url_from_map {
            exit_address = exit
        }
    }
	return strings.TrimSpace(exit_address)
}

func tcpProxy(w net.Conn, req *http.Request, host string, ingress bool, skey string, originkey string, fname string) {
	// start a new TCP connection with the server
	conn, err := net.Dial("tcp", host)
	if err != nil {
		sendError(w, SERVERROR)
		return
	}
	defer conn.Close()

	// Send the serialized request to the server
	err = req.Write(conn)
	if err != nil {
		sendError(w, SERVERROR)
		return
	}

	// read from the server in a loop, sending the
	// response back to the client
	connbuf := bufio.NewReader(conn)
	var buf []byte
	partial := false
	for {
		str, err := connbuf.ReadBytes('\n')
		buf = append(buf, str...)
		if err != nil {
			if err != io.EOF {
				if !partial {
					sendError(w, SERVERROR)
				}
				return
			}

			// [Annie] response formatting (strip all response headers) TODO: check what format the client proxy gets the response from exit in (are there response headers?)
			temp := strings.Split(string(buf), "Server: lighttpd/1.4.33")
			var temp2 string
            //var plain_text string
            var plain_text_bytes []byte
            var headers string
			if len(temp) > 1 {
                headers = string(temp[0] + "Server: lighttpd/1.4.33\n\n")
				temp2 = strings.TrimSpace(temp[1])
			} else {
				temp2 = strings.TrimSpace(temp[0])
			}

            if ingress {
                time1 := int64(time.Now().UnixNano())
                // [Annie] decrypt content with session key
				enc_text_bytes, _ := base64.StdEncoding.DecodeString(temp2)
                plain_text_bytes = []byte(decryptAES(string(enc_text_bytes), skey))
				// [Annie] buf should now hold new plaintext content
                whole_resp := string(headers + string(plain_text_bytes))
				buf = []byte(whole_resp)
                time2 := int64(time.Now().UnixNano())
                fmt.Printf("8,%d,%s\n", time2-time1,fname)
            } else {
                time1 := int64(time.Now().UnixNano())
                // [Annie] decrypt content with shared key
                dec1, err := base64.StdEncoding.DecodeString(temp2)
                if err != nil {
                    log.Println(err)
                }
                dec2 := string(dec1)
				plain_text := decryptAES(dec2, originkey)
                time2 := int64(time.Now().UnixNano())
                fmt.Printf("6,%d,%s\n", time2-time1,fname)
                // [Annie] encrypt content with session key
				new_cipher_text := base64.StdEncoding.EncodeToString([]byte(encryptAES(plain_text, skey)))
				// [Annie] buf should now hold new encrypted content
                whole_resp := string(headers + new_cipher_text)
				buf = []byte(whole_resp)
                time3 := int64(time.Now().UnixNano())
                fmt.Printf("7,%d,%s\n", time3-time2,fname)
            }

			w.Write(buf)
			return
		}
		if len(buf) >= MAX_BUFFER {
			_, err := w.Write(buf)
			if err != nil {
				return
			}
			buf = buf[:0]
			partial = true
		}
	}

}

func handleRequest(w net.Conn, t int64, exit_map map[string]string, pub *rsa.PublicKey, shared_key string) {
	// close the connection with the socket once finished handling request
	defer w.Close()

	// read the requst from the client
	r := bufio.NewReader(w)
	req, err := http.ReadRequest(r)

	// error checking
	if err != nil {
		sendError(w, SERVERROR)
		return
	}
	if req.Method != "GET" {
		sendError(w, BADREQ)
		return
	}

	// modify the request as per the proxy specifications
	req.Header.Set("Connection", "close")
	req.Header.Set("Host", req.Host)
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1

	ingress := false

    filename := strings.Replace(req.URL.Path, "/", "", -1)

	// Check for presence of X-OCDN header - ingress or egress
	enc_session_key := req.Header.Get("X-OCDN")

	if enc_session_key == "" {
        // Ingress - need to set session key and redirect connection to exit proxy

        ingress = true
        //pub := readPublicKey()
        session_key := base64.StdEncoding.EncodeToString([]byte(generateSessionKey()))
        time2 := int64(time.Now().UnixNano())
        fmt.Printf("1,%d,%s\n", time2-t,filename)

        enc_skey := base64.StdEncoding.EncodeToString([]byte(encryptAsymmetric(session_key, pub)))
        req.Header.Set("X-OCDN", enc_skey)
        time3 := int64(time.Now().UnixNano())
        fmt.Printf("2,%d,%s\n", time3-time2,filename)

        // decode session key for use later
        session_key_bytes, _ := base64.StdEncoding.DecodeString(session_key)
        decoded_session_key := string(session_key_bytes)

        time4 := int64(time.Now().UnixNano())
        exit_proxy := exit_map[string(req.Host + req.URL.Path)]
        //exit_proxy := getExitProxy(string(req.Host + req.URL.Path))
        time5 := int64(time.Now().UnixNano())
        fmt.Printf("3,%d,%s\n", time5-time4,filename)
		tcpProxy(w, req, exit_proxy, ingress, decoded_session_key, "", filename)
	} else {

		// Egress - open connection to actual server and encrypt / decrypt content
		// get the hostname of the server
		var newHost string
		if strings.Contains(req.Host, ":") {
			newHost = req.Host
		} else {
			newHost = req.Host + ":http"
		}

		// [Annie] Get own private key

		priv_key := readPrivateKey()

		if priv_key == nil {
			log.Println("Private key not found")
			return
		}

        time1 := int64(time.Now().UnixNano())
        // [Annie] look up shared key in file
		//key_bytes, _ := ioutil.ReadFile(string("../keys/shared_key_" + req.URL.Path[1:]))
		//shared_key := string(key_bytes)
        time2 := int64(time.Now().UnixNano())
        fmt.Printf("4,%d,%s\n", time2-time1,filename)

        t := strings.Replace(req.URL.Path, "/", "", -1)
        // [Annie] Mangle the URL
		enc_host := generateMAC(t, shared_key)
        clean_enc_host := strings.Replace(enc_host, "/", "-", -1)
		c := "/obf/"
		req.URL.Path = c + string(clean_enc_host)
        time3 := int64(time.Now().UnixNano())
        fmt.Printf("5,%d,%s\n", time3-time2,filename)

        decode_key_bytes, err := base64.StdEncoding.DecodeString(enc_session_key)
        if err != nil {
            log.Println(err)
        }
        decode_key_str := string(decode_key_bytes)
        session_key := decryptAsymmetric(decode_key_str, priv_key)
        session_key2, err2 := base64.StdEncoding.DecodeString(session_key)
        if err2 != nil {
            log.Println(err)
        }
        session_key2_str := string(session_key2)

        if session_key2_str == "" {
			log.Fatal("Session key empty!")
            return
		} else {
			tcpProxy(w, req, newHost, ingress, session_key2_str, shared_key,filename)
		}
	}

}

func main() {
	//Parse command line arguments
	if len(os.Args) != 2 {
		log.Fatal("Usage: server <port-number>")
	}
	portStr := ":" + os.Args[1]

    // read in url_exit_mapping
    exit_mapping_data := make(map[string]string)
    url_exit_file, err := os.Open("url_exit_mapping.txt")
    if err != nil {
        log.Fatal(err)
    }
    defer url_exit_file.Close()
    scanner := bufio.NewScanner(url_exit_file)
    for scanner.Scan() {
        s := strings.Split(scanner.Text(), ",")
        url_from_map, exit := s[0], s[1]
        exit_mapping_data[url_from_map] = strings.TrimSpace(exit)
    }

    // read in public key(s)
    pub := readPublicKey()

    // read in shared key(s)
    key_bytes, _ := ioutil.ReadFile(string("../keys/shared_key_9991.txt"))
	shared_key := string(key_bytes)

	// listen on socket
	ln, err := net.Listen("tcp", portStr)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

	for {
		// accept client connections
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Accept: ", err)
			continue
		}
        // get system time for overhead calculations
		t := int64(time.Now().UnixNano())

		// start goroutine to handle client
		go handleRequest(conn, t, exit_mapping_data, pub, shared_key)
	}
}
