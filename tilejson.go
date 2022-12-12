package main

import (
	"encoding/json"
	"strconv"
)

type TileJSON struct {
	Tilejson     string        `json:"tilejson"`
	Name         string        `json:"name,omitempty"`
	Description  string        `json:"description,omitempty"`
	Version      string        `json:"version,omitempty"`
	Attribution  string        `json:"attribution,omitempty"`
	Tiles        []string      `json:"tiles"`
	Minzoom      int           `json:"minzoom,omitempty"`
	Maxzoom      int           `json:"maxzoom,omitempty"`
	Bounds       []float64     `json:"bounds,omitempty"`
	Center       []float64     `json:"center,omitempty"`
	VectorLayers []VectorLayer `json:"vector_layers,omitempty"`
}

type VectorLayer struct {
	Id     string            `json:"id,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
}

func valueOrDefaultString(metadata *map[string]string, key string, defaultValue string) string {
	if value, ok := (*metadata)[key]; ok {
		return value
	}
	return defaultValue
}

func valueOrDefaultInt(metadata *map[string]string, key string, defaultValue int) int {
	if value, ok := (*metadata)[key]; ok {
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return defaultValue
		}
		return intVal
	}
	return defaultValue
}

func FromMBTiles(tileurl string, metadata map[string]string) TileJSON {

	var vlayers []VectorLayer
	err := json.Unmarshal([]byte(metadata["vector_layers"]), &vlayers)
	if err != nil {
		vlayers = []VectorLayer{}
	}

	tilejson := TileJSON{
		Tilejson:     "3.0.0",
		Name:         metadata["name"],
		Description:  metadata["description"],
		Version:      metadata["version"],
		Attribution:  metadata["attribution"],
		Tiles:        []string{tileurl},
		Minzoom:      valueOrDefaultInt(&metadata, "minzoom", 0),
		Maxzoom:      valueOrDefaultInt(&metadata, "maxzoom", 14),
		Bounds:       []float64{-180, -85.0511, 180, 85.0511},
		Center:       []float64{0, 0, 2},
		VectorLayers: vlayers,
	}
	return tilejson
}
