package nest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

/*
SetFanTimerActive sets the fan timer on or off
https://developer.nest.com/documentation/api#fan_timer_active

	t.SetFanTimerActive(true)
*/
func (t *Thermostat) SetFanTimerActive(setting bool) *APIError {
	request := make(map[string]bool)
	request["fan_timer_active"] = setting
	body, _ := json.Marshal(request)
	return t.setThermostat(body)
}

/*
SetHvacMode sets the HvacMode when a thermostat may heat and cool
https://developer.nest.com/documentation/api#hvac_mode

	t.SetHvacMode(Cool)
*/
func (t *Thermostat) SetHvacMode(mode int) *APIError {
	requestMode := make(map[string]string)
	switch mode {
	case Cool:
		requestMode["hvac_mode"] = "cool"
	case Heat:
		requestMode["hvac_mode"] = "heat"
	case HeatCool:
		requestMode["hvac_mode"] = "heat-cool"
	case Off:
		requestMode["hvac_mode"] = "off"
	default:
		return generateAPIError("Invalid HvacMode requested - must be cool, heat, heat-cool or off")
	}
	body, _ := json.Marshal(requestMode)
	return t.setThermostat(body)
}

/*
SetTargetTempC sets the thermostat to an intended temp in celcius
https://developer.nest.com/documentation/api#target_temperature_c

	t.SetTargetTempC(28.5)
*/
func (t *Thermostat) SetTargetTempC(temp float32) *APIError {
	if temp < 9 || temp > 32 {
		return generateAPIError("Temperature must be between 9 and 32 Celcius")
	}
	tempRequest := make(map[string]float32)
	tempRequest["target_temperature_c"] = temp
	body, _ := json.Marshal(tempRequest)
	return t.setThermostat(body)
}

/*
SetTargetTempF sets the thermostat to an intended temp in farenheit
https://developer.nest.com/documentation/api#target_temperature_f

	t.SetTargetTempF(78)
*/
func (t *Thermostat) SetTargetTempF(temp int) *APIError {
	if temp < 50 || temp > 90 {
		return generateAPIError("Temperature must be between 50 and 90 Farenheit")
	}
	request := make(map[string]int)
	request["target_temperature_f"] = temp
	body, _ := json.Marshal(request)
	return t.setThermostat(body)
}

/*
SetTargetTempHighLowC sets the high target temp in celcius when HvacMode is HeatCool
https://developer.nest.com/documentation/api#target_temperature_high_c
https://developer.nest.com/documentation/api#target_temperature_low_c

	t.SetTargetTempHighLowF(75, 65)
*/
func (t *Thermostat) SetTargetTempHighLowC(high float32, low float32) *APIError {
	if high < low {
		return generateAPIError("The high temperature must be greater than the low temperature")
	}
	request := make(map[string]float32)
	request["target_temperature_high_c"] = high
	request["target_temperature_low_c"] = low
	body, _ := json.Marshal(request)
	return t.setThermostat(body)
}

/*
SetTargetTempHighLowF sets the high target temp in farenheit when HvacMode is HeatCool
https://developer.nest.com/documentation/api#target_temperature_high_f
https://developer.nest.com/documentation/api#target_temperature_low_f

	t.SetTargetTempHighLowF(75, 65)
*/
func (t *Thermostat) SetTargetTempHighLowF(high int, low int) *APIError {
	if high < low {
		return generateAPIError("The high temperature must be greater than the low temperature")
	}
	request := make(map[string]int)
	request["target_temperature_high_f"] = high
	request["target_temperature_low_f"] = low
	body, _ := json.Marshal(request)
	return t.setThermostat(body)
}

// setThermostat sends the request to the Nest REST API
func (t *Thermostat) setThermostat(body []byte) *APIError {
	url := t.Client.RedirectURL + "/devices/thermostats/" + t.DeviceID + "?auth=" + t.Client.Token
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
		t.Client.RedirectURL = resp.Request.URL.Scheme + "://" + resp.Request.URL.Host
		url := t.Client.RedirectURL + "/devices/thermostats/" + t.DeviceID + "?auth=" + t.Client.Token
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
		thermostat := &Thermostat{}
		json.Unmarshal(body, thermostat)
		return nil
	}
	apiError := &APIError{}
	json.Unmarshal(body, apiError)
	apiError = generateAPIError(apiError.Error)
	apiError.Status = resp.Status
	apiError.StatusCode = resp.StatusCode
	return apiError
}

// generateAPIError generates an error to return when an API call is invalid
func generateAPIError(description string) *APIError {
	apiError := &APIError{
		Error:       "api_error",
		Description: description,
	}
	return apiError
}
