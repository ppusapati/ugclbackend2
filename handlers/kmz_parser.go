package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

// KML Geometry types
type Point struct {
	Coordinates string `xml:"coordinates"`
}

type LineString struct {
	Coordinates string `xml:"coordinates"`
}

type LinearRing struct {
	Coordinates string `xml:"coordinates"`
}

type Polygon struct {
	OuterBoundary struct {
		LinearRing LinearRing `xml:"LinearRing"`
	} `xml:"outerBoundaryIs"`
	InnerBoundaries []struct {
		LinearRing LinearRing `xml:"LinearRing"`
	} `xml:"innerBoundaryIs"`
}

type MultiGeometry struct {
	Points      []Point      `xml:"Point"`
	LineStrings []LineString `xml:"LineString"`
	Polygons    []Polygon    `xml:"Polygon"`
}

// ExtendedData represents KML extended data
type ExtendedData struct {
	Data []struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value"`
	} `xml:"Data"`
	SchemaData []struct {
		SchemaURL    string `xml:"schemaUrl,attr"`
		SimpleData   []struct {
			Name  string `xml:"name,attr"`
			Value string `xml:",chardata"`
		} `xml:"SimpleData"`
	} `xml:"SchemaData"`
}

type Placemark struct {
	XMLName       xml.Name       `xml:"Placemark"`
	ID            string         `xml:"id,attr"`
	Name          string         `xml:"name"`
	Description   string         `xml:"description"`
	StyleUrl      string         `xml:"styleUrl"`
	ExtendedData  *ExtendedData  `xml:"ExtendedData"`
	Point         *Point         `xml:"Point"`
	LineString    *LineString    `xml:"LineString"`
	Polygon       *Polygon       `xml:"Polygon"`
	MultiGeometry *MultiGeometry `xml:"MultiGeometry"`
}

type Folder struct {
	XMLName    xml.Name    `xml:"Folder"`
	Name       string      `xml:"name"`
	Placemarks []Placemark `xml:"Placemark"`
	Folders    []Folder    `xml:"Folder"`
}

type Document struct {
	XMLName    xml.Name    `xml:"Document"`
	Name       string      `xml:"name"`
	Placemarks []Placemark `xml:"Placemark"`
	Folders    []Folder    `xml:"Folder"`
}

type KML struct {
	XMLName  xml.Name `xml:"kml"`
	Document Document `xml:"Document"`
}

// KMZParser handles KMZ file parsing
type KMZParser struct{}

// NewKMZParser creates a new KMZ parser instance
func NewKMZParser() *KMZParser {
	return &KMZParser{}
}

// ExtractKML extracts KML content from KMZ file
func (p *KMZParser) ExtractKML(kmzData []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(kmzData), int64(len(kmzData)))
	if err != nil {
		return nil, fmt.Errorf("failed to open KMZ archive: %w", err)
	}

	for _, f := range reader.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".kml") {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open KML file: %w", err)
			}
			defer rc.Close()

			return io.ReadAll(rc)
		}
	}

	return nil, fmt.Errorf("no KML file found in KMZ archive")
}

// ParseKML parses KML XML data
func (p *KMZParser) ParseKML(kmlData []byte) (*KML, error) {
	var kml KML
	if err := xml.Unmarshal(kmlData, &kml); err != nil {
		return nil, fmt.Errorf("failed to parse KML: %w", err)
	}
	return &kml, nil
}

// ParseCoordinates parses KML coordinate string into [][]float64
func (p *KMZParser) ParseCoordinates(s string) [][]float64 {
	coords := [][]float64{}
	for _, pair := range strings.Fields(strings.TrimSpace(s)) {
		parts := strings.Split(pair, ",")
		if len(parts) >= 2 {
			var lon, lat, ele float64
			fmt.Sscanf(parts[0], "%f", &lon)
			fmt.Sscanf(parts[1], "%f", &lat)
			if len(parts) >= 3 {
				fmt.Sscanf(parts[2], "%f", &ele)
				coords = append(coords, []float64{lon, lat, ele})
			} else {
				coords = append(coords, []float64{lon, lat})
			}
		}
	}
	return coords
}

// ExtractProperties extracts properties from ExtendedData
func (p *KMZParser) ExtractProperties(pm *Placemark) map[string]interface{} {
	props := map[string]interface{}{
		"name":        pm.Name,
		"description": pm.Description,
	}

	if pm.ID != "" {
		props["id"] = pm.ID
	}

	if pm.StyleUrl != "" {
		props["styleUrl"] = pm.StyleUrl
	}

	// Extract ExtendedData
	if pm.ExtendedData != nil {
		for _, data := range pm.ExtendedData.Data {
			props[data.Name] = data.Value
		}
		for _, schemaData := range pm.ExtendedData.SchemaData {
			for _, simpleData := range schemaData.SimpleData {
				props[simpleData.Name] = simpleData.Value
			}
		}
	}

	return props
}

// ConvertPlacemarkToGeoJSON converts a single placemark to GeoJSON features
func (p *KMZParser) ConvertPlacemarkToGeoJSON(pm *Placemark) []*geojson.Feature {
	features := []*geojson.Feature{}
	props := p.ExtractProperties(pm)

	// Handle Point
	if pm.Point != nil {
		coords := p.ParseCoordinates(pm.Point.Coordinates)
		if len(coords) > 0 {
			point := orb.Point{coords[0][0], coords[0][1]}
			feature := geojson.NewFeature(point)
			feature.Properties = props
			if len(coords[0]) >= 3 {
				feature.Properties["elevation"] = coords[0][2]
			}
			features = append(features, feature)
		}
	}

	// Handle LineString
	if pm.LineString != nil {
		coords := p.ParseCoordinates(pm.LineString.Coordinates)
		if len(coords) > 0 {
			lineString := make(orb.LineString, len(coords))
			for i, coord := range coords {
				lineString[i] = orb.Point{coord[0], coord[1]}
			}
			feature := geojson.NewFeature(lineString)
			feature.Properties = props
			features = append(features, feature)
		}
	}

	// Handle Polygon
	if pm.Polygon != nil {
		coords := p.ParseCoordinates(pm.Polygon.OuterBoundary.LinearRing.Coordinates)
		if len(coords) > 0 {
			ring := make(orb.Ring, len(coords))
			for i, coord := range coords {
				ring[i] = orb.Point{coord[0], coord[1]}
			}
			polygon := orb.Polygon{ring}

			// Handle inner boundaries (holes)
			for _, inner := range pm.Polygon.InnerBoundaries {
				innerCoords := p.ParseCoordinates(inner.LinearRing.Coordinates)
				if len(innerCoords) > 0 {
					innerRing := make(orb.Ring, len(innerCoords))
					for i, coord := range innerCoords {
						innerRing[i] = orb.Point{coord[0], coord[1]}
					}
					polygon = append(polygon, innerRing)
				}
			}

			feature := geojson.NewFeature(polygon)
			feature.Properties = props
			features = append(features, feature)
		}
	}

	// Handle MultiGeometry
	if pm.MultiGeometry != nil {
		// Points
		for _, pt := range pm.MultiGeometry.Points {
			coords := p.ParseCoordinates(pt.Coordinates)
			if len(coords) > 0 {
				point := orb.Point{coords[0][0], coords[0][1]}
				feature := geojson.NewFeature(point)
				feature.Properties = props
				if len(coords[0]) >= 3 {
					feature.Properties["elevation"] = coords[0][2]
				}
				features = append(features, feature)
			}
		}

		// LineStrings
		for _, ln := range pm.MultiGeometry.LineStrings {
			coords := p.ParseCoordinates(ln.Coordinates)
			if len(coords) > 0 {
				lineString := make(orb.LineString, len(coords))
				for i, coord := range coords {
					lineString[i] = orb.Point{coord[0], coord[1]}
				}
				feature := geojson.NewFeature(lineString)
				feature.Properties = props
				features = append(features, feature)
			}
		}

		// Polygons
		for _, pg := range pm.MultiGeometry.Polygons {
			coords := p.ParseCoordinates(pg.OuterBoundary.LinearRing.Coordinates)
			if len(coords) > 0 {
				ring := make(orb.Ring, len(coords))
				for i, coord := range coords {
					ring[i] = orb.Point{coord[0], coord[1]}
				}
				polygon := orb.Polygon{ring}
				feature := geojson.NewFeature(polygon)
				feature.Properties = props
				features = append(features, feature)
			}
		}
	}

	return features
}

// ProcessFolder recursively processes folders and extracts features
func (p *KMZParser) ProcessFolder(folder *Folder) []*geojson.Feature {
	features := []*geojson.Feature{}

	// Add folder name to properties
	for _, pm := range folder.Placemarks {
		pmFeatures := p.ConvertPlacemarkToGeoJSON(&pm)
		for _, f := range pmFeatures {
			if folder.Name != "" {
				f.Properties["folder"] = folder.Name
			}
			features = append(features, f)
		}
	}

	// Process nested folders
	for _, subFolder := range folder.Folders {
		subFeatures := p.ProcessFolder(&subFolder)
		features = append(features, subFeatures...)
	}

	return features
}

// ParseKMZToGeoJSON parses KMZ file and returns GeoJSON FeatureCollection
func (p *KMZParser) ParseKMZToGeoJSON(kmzData []byte) (*geojson.FeatureCollection, error) {
	// Extract KML from KMZ
	kmlData, err := p.ExtractKML(kmzData)
	if err != nil {
		return nil, err
	}

	// Parse KML
	kml, err := p.ParseKML(kmlData)
	if err != nil {
		return nil, err
	}

	// Convert to GeoJSON
	fc := geojson.NewFeatureCollection()

	// Process document-level placemarks
	for _, pm := range kml.Document.Placemarks {
		features := p.ConvertPlacemarkToGeoJSON(&pm)
		fc.Features = append(fc.Features, features...)
	}

	// Process folders
	for _, folder := range kml.Document.Folders {
		features := p.ProcessFolder(&folder)
		fc.Features = append(fc.Features, features...)
	}

	return fc, nil
}

// ParsedKMZData represents the structured data extracted from KMZ
type ParsedKMZData struct {
	GeoJSON      *geojson.FeatureCollection `json:"geojson"`
	Zones        []ZoneData                 `json:"zones"`
	Nodes        []NodeData                 `json:"nodes"`
	Labels       []LabelData                `json:"labels"`
	TotalFeatures int                       `json:"total_features"`
}

// ZoneData represents a zone extracted from KMZ
type ZoneData struct {
	Name       string                 `json:"name"`
	Code       string                 `json:"code,omitempty"`
	Label      string                 `json:"label,omitempty"`
	GeometryType string               `json:"geometry_type"`
	GeoJSON    map[string]interface{} `json:"geojson"`
	Properties map[string]interface{} `json:"properties"`
}

// NodeData represents a node (point) extracted from KMZ
type NodeData struct {
	Name       string                 `json:"name"`
	Code       string                 `json:"code,omitempty"`
	Label      string                 `json:"label,omitempty"`
	NodeType   string                 `json:"node_type"` // inferred from name/properties
	Latitude   float64                `json:"latitude"`
	Longitude  float64                `json:"longitude"`
	Elevation  float64                `json:"elevation,omitempty"`
	GeoJSON    map[string]interface{} `json:"geojson"`
	Properties map[string]interface{} `json:"properties"`
}

// LabelData represents a label/placemark
type LabelData struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	GeoJSON    map[string]interface{} `json:"geojson"`
	Properties map[string]interface{} `json:"properties"`
}

// ParseKMZToStructuredData parses KMZ and categorizes data into zones, nodes, and labels
func (p *KMZParser) ParseKMZToStructuredData(kmzData []byte) (*ParsedKMZData, error) {
	fc, err := p.ParseKMZToGeoJSON(kmzData)
	if err != nil {
		return nil, err
	}

	result := &ParsedKMZData{
		GeoJSON:      fc,
		Zones:        []ZoneData{},
		Nodes:        []NodeData{},
		Labels:       []LabelData{},
		TotalFeatures: len(fc.Features),
	}

	for _, feature := range fc.Features {
		geomType := feature.Geometry.GeoJSONType()
		name := ""
		if n, ok := feature.Properties["name"].(string); ok {
			name = n
		}

		// Convert feature to map for storage
		featureBytes, _ := json.Marshal(feature)
		var featureMap map[string]interface{}
		json.Unmarshal(featureBytes, &featureMap)

		switch geomType {
		case "Point":
			// This is a node
			point := feature.Geometry.(orb.Point)
			nodeType := p.InferNodeType(name, feature.Properties)

			nodeData := NodeData{
				Name:       name,
				NodeType:   nodeType,
				Latitude:   point.Lat(),
				Longitude:  point.Lon(),
				GeoJSON:    featureMap,
				Properties: feature.Properties,
			}

			if code, ok := feature.Properties["code"].(string); ok {
				nodeData.Code = code
			}
			if label, ok := feature.Properties["label"].(string); ok {
				nodeData.Label = label
			}
			if ele, ok := feature.Properties["elevation"].(float64); ok {
				nodeData.Elevation = ele
			}

			result.Nodes = append(result.Nodes, nodeData)

		case "Polygon", "MultiPolygon":
			// This is a zone
			zoneData := ZoneData{
				Name:         name,
				GeometryType: geomType,
				GeoJSON:      featureMap,
				Properties:   feature.Properties,
			}

			if code, ok := feature.Properties["code"].(string); ok {
				zoneData.Code = code
			}
			if label, ok := feature.Properties["label"].(string); ok {
				zoneData.Label = label
			}

			result.Zones = append(result.Zones, zoneData)

		case "LineString", "MultiLineString":
			// This could be a path or boundary - store as label
			labelData := LabelData{
				Name:       name,
				Type:       "line",
				GeoJSON:    featureMap,
				Properties: feature.Properties,
			}
			result.Labels = append(result.Labels, labelData)
		}
	}

	return result, nil
}

// InferNodeType infers node type from name and properties
func (p *KMZParser) InferNodeType(name string, properties map[string]interface{}) string {
	nameLower := strings.ToLower(name)

	// Check properties first
	if nodeType, ok := properties["node_type"].(string); ok {
		return nodeType
	}
	if nodeType, ok := properties["type"].(string); ok {
		return nodeType
	}

	// Infer from name
	if strings.Contains(nameLower, "start") || strings.Contains(nameLower, "begin") {
		return "start"
	}
	if strings.Contains(nameLower, "stop") || strings.Contains(nameLower, "end") {
		return "stop"
	}
	if strings.Contains(nameLower, "waypoint") || strings.Contains(nameLower, "checkpoint") {
		return "waypoint"
	}

	// Default to waypoint
	return "waypoint"
}
