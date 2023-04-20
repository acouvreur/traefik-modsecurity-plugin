package traefik_modsecurity_plugin

import (
	"net/http"
	"net/url"
)

func contains(methods []string, method string) bool {
	for _, m := range methods {
		if m == method {
			return true
		}
	}
	return false
}

// Check if the request is a websocket upgrade
func isWebsocket(req *http.Request) bool {
	for _, header := range req.Header["Upgrade"] {
		if header == "websocket" {
			return true
		}
	}
	return false
}

func removeTrackingParams(inputURL string) string {
	trackingParams := []string{
		"fbclid",                // Facebook
		"gclid",                 // Google Ads / Google Analytics
		"gclsrc",                // Google DoubleClick
		"dclid",                 // Old DoubleClick
		"utm_content",           // Google Analytics
		"utm_term",              // Google Analytics
		"utm_campaign",          // Google Analytics
		"utm_medium",            // Google Analytics
		"utm_source",            // Google Analytics
		"utm_id",                // Google Analytics
		"_ga",                   // Google Analytics
		"mc_cid",                // Mailchimp
		"mc_eid",                // Mailchimp
		"_bta_tid",              // Bronto
		"_bta_c",                // Bronto
		"trk_contact",           // Listrak
		"trk_msg",               // Listrak
		"trk_module",            // Listrak
		"trk_sid",               // Listrak
		"gdfms",                 // GoDataFeed
		"gdftrk",                // GoDataFeed
		"gdffi",                 // GoDataFeed
		"_ke",                   // Klaviyo
		"redirect_log_mongo_id", // Springbot
		"redirect_mongo_id",     // Springbot
		"sb_referer_host",       // Springbot
		"mkwid",                 // Marin
		"pcrid",                 // Marin
		"ef_id",                 // Adobe Advertising Cloud
		"s_kwcid",               // Adobe Analytics
		"msclkid",               // Microsoft Advertising
		"dm_i",                  // dotdigital
		"epik",                  // Pinterest
		"pk_campaign",           // Piwik
		"pk_kwd",                // Piwik
		"pk_keyword",            // Piwik
		"piwik_campaign",        // Piwik
		"piwik_kwd",             // Piwik
		"piwik_keyword",         // Piwik
		"mtm_campaign",          // Matomo
		"mtm_keyword",           // Matomo
		"mtm_source",            // Matomo
		"mtm_medium",            // Matomo
		"mtm_content",           // Matomo
		"mtm_cid",               // Matomo
		"mtm_group",             // Matomo
		"mtm_placement",         // Matomo
		"matomo_campaign",       // Matomo
		"matomo_keyword",        // Matomo
		"matomo_source",         // Matomo
		"matomo_medium",         // Matomo
		"matomo_content",        // Matomo
		"matomo_cid",            // Matomo
		"matomo_group",          // Matomo
		"matomo_placement",      // Matomo
		"hsa_cam",               // Hubspot
		"hsa_grp",               // Hubspot
		"hsa_mt",                // Hubspot
		"hsa_src",               // Hubspot
		"hsa_ad",                // Hubspot
		"hsa_acc",               // Hubspot
		"hsa_net",               // Hubspot
		"hsa_kw",                // Hubspot
		"hsa_tgt",               // Hubspot
		"hsa_ver",               // Hubspot
	}

	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return inputURL
	}

	query := parsedURL.Query()

	for _, param := range trackingParams {
		query.Del(param)
	}

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}
