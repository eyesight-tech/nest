package nest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

/*
Structures returns a map of structures
https://developer.nest.com/documentation/api#structures

	structures := client.Structures()
*/
func (c *Client) Structures() (map[string]*Structure, *APIError) {
	resp, err := c.getStructures(NoStream)
	if err != nil {
		return nil, &APIError{
			Error:       "devices_error",
			Description: err.Error(),
		}
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, &APIError{
			Error:       "body_read_error",
			Description: err.Error(),
		}
	}
	if resp.StatusCode != 200 {
		apiError := &APIError{}
		json.Unmarshal(body, apiError)
		return nil, apiError
	}
	structures := make(map[string]*Structure)
	err = json.Unmarshal(body, &structures)
	c.associateClientToStructures(structures)
	return structures, nil
}

/*
Structures Stream emits events from the Nest structures REST streaming API

	client.StructuresStream(func(event map[string]*Structure) {
		fmt.Println(event)
	})
*/
func (c *Client) StructuresStream(callback func(structures map[string]*Structure, err error)) {
	c.setRedirectURL()
	for {
		c.streamStructures(callback)
	}
}

/*
SetAway sets the away status of a structure
https://developer.nest.com/documentation/api#away

	s.SetAway(nest.Away)
*/
func (s *Structure) SetAway(mode int) *APIError {
	requestMode := make(map[string]string)
	switch mode {
	case Home:
		requestMode["away"] = "home"
	case Away:
		requestMode["away"] = "away"
	case AutoAway:
		requestMode["away"] = "auto-away"
	default:
		return generateAPIError("Invalid Away requested - must be home, away or auto-away")
	}
	body, _ := json.Marshal(requestMode)
	return s.setStructure(body)
}

/*
SetETA sets the ETA for the Nest API
https://developer.nest.com/documentation/eta-reference

*/
func (s *Structure) SetETA(tripID string, begin time.Time, end time.Time) *APIError {
	apiErr := checkTimes(begin, end)
	if apiErr != nil {
		return apiErr
	}
	eta := &ETA{
		TripID: tripID,
		EstimatedArrivalWindowBegin: begin,
		EstimatedArrivalWindowEnd:   end,
	}
	data, _ := json.Marshal(eta)
	req, _ := http.NewRequest("PUT", s.Client.RedirectURL+"/structures/"+s.StructureID+"/eta.json?auth="+s.Client.Token, bytes.NewBuffer(data))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		apiError := &APIError{
			Error:       "http_error",
			Description: err.Error(),
		}
		return apiError
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		apiError := &APIError{}
		json.Unmarshal(body, apiError)
		apiError = generateAPIError(apiError.Error)
		apiError.Status = resp.Status
		apiError.StatusCode = resp.StatusCode
		return apiError
	}
	return nil
}

// checkTimes ensure the times provided are set properly for the Nest API
func checkTimes(begin time.Time, end time.Time) *APIError {
	if begin.Before(time.Now()) {
		apiError := &APIError{
			Error:       "eta_error",
			Description: "The begin time must be greater than the time now",
		}
		return apiError
	}
	if end.Before(begin) {
		apiError := &APIError{
			Error:       "eta_error",
			Description: "The end time must be greater than the begin time",
		}
		return apiError
	}
	return nil
}

// streamStructures connects to the stream, following the redirect and then watches the stream
func (c *Client) streamStructures(callback func(structures map[string]*Structure, err error)) {
	resp, err := c.getStructures(Stream)
	if err != nil {
		callback(nil, err)
		return
	}
	defer resp.Body.Close()
	c.watchStructuresStream(resp, callback)
}

// watchStructuresStream grabs the data off the stream, parses them and invokes the callback
func (c *Client) watchStructuresStream(resp *http.Response, callback func(structures map[string]*Structure, err error)) {
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		value := parseStreamData(line)
		if value != "" {
			structuresEvent := &StructuresEvent{}
			json.Unmarshal([]byte(value), structuresEvent)
			if structuresEvent.Data != nil {
				c.associateClientToStructures(structuresEvent.Data)
				callback(structuresEvent.Data, nil)
			}
		}
	}
}

// getStructures does an HTTP get
func (c *Client) getStructures(action int) (*http.Response, error) {
	if c.RedirectURL == "" {
		req, _ := http.NewRequest("GET", c.APIURL+"/structures.json?auth="+c.Token, nil)
		resp, err := http.DefaultClient.Do(req)
		if resp.Request.URL != nil {
			c.RedirectURL = resp.Request.URL.Scheme + "://" + resp.Request.URL.Host
		}
		return resp, err
	}

	req, _ := http.NewRequest("GET", c.RedirectURL+"/structures.json?auth="+c.Token, nil)
	if action == Stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	resp, err := http.DefaultClient.Do(req)
	return resp, err
}

// setStructure sends the request to the Nest REST API
func (s *Structure) setStructure(body []byte) *APIError {

	url := s.Client.RedirectURL + "/structures/" + s.StructureID + "?auth=" + s.Client.Token
	client := &http.Client{}
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error before redirect! ", err)
		apiError := &APIError{
			Error:       "http_error",
			Description: err.Error(),
		}
		return apiError
	}
	if resp.StatusCode == 307 {
		fmt.Println("307 after request! ")
		s.Client.RedirectURL = resp.Request.URL.Scheme + "://" + resp.Request.URL.Host
		url := s.Client.RedirectURL + "/structures/" + s.StructureID + "?auth=" + s.Client.Token
		req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			fmt.Println("Error after redirect! ", err)
			apiError := &APIError{
				Error:       "http_error",
				Description: err.Error(),
			}
			return apiError
		}
		resp = response
	}
	body, _ = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		structure := &Structure{}
		json.Unmarshal(body, structure)
		return nil
	}
	apiError := &APIError{}
	json.Unmarshal(body, apiError)
	apiError = generateAPIError(apiError.Error)
	apiError.Status = resp.Status
	apiError.StatusCode = resp.StatusCode
	return apiError
}

// associateClientToStructures ensures each structure knows its client details
func (c *Client) associateClientToStructures(structures map[string]*Structure) {
	for _, value := range structures {
		value.Client = c
	}
}
