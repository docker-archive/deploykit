/*
 * This file is part of the libvirt-go-xml project
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * Copyright (C) 2016 Red Hat, Inc.
 *
 */

package libvirtxml

import (
	"encoding/xml"
)

type CapsHostCPUTopology struct {
	Sockets int `xml:"sockets,attr"`
	Cores   int `xml:"cores,attr"`
	Threads int `xml:"threads,attr"`
}

type CapsHostCPUFeature struct {
	Name string `xml:"name,attr"`
}

type CapsHostCPUPageSize struct {
	Size int    `xml:"size,attr"`
	Unit string `xml:"unit,attr"`
}

type CapsHostCPU struct {
	Arch      string                `xml:"arch"`
	Model     string                `xml:"model"`
	Vendor    string                `xml:"vendor"`
	Topology  CapsHostCPUTopology   `xml:"topology"`
	Features  []CapsHostCPUFeature  `xml:"feature"`
	PageSizes []CapsHostCPUPageSize `xml:"pages"`
}

type CapsHostNUMAMemory struct {
	Size uint64 `xml:"size,attr"`
	Unit string `xml:"unit,attr"`
}

type CapsHostNUMAPageInfo struct {
	Size  int    `xml:"size,attr"`
	Unit  string `xml:"unit,attr"`
	Count uint64 `xml:",chardata"`
}

type CapsHostNUMACPU struct {
	ID       int    `xml:"id,attr"`
	SocketID int    `xml:"socket_id,attr"`
	CoreID   int    `xml:"core_id,attr"`
	Siblings string `xml:"siblings,attr"`
}

type CapsHostNUMADistance struct {
	ID    int `xml:"id,attr"`
	Value int `xml:"value,attr"`
}

type CapsHostNUMACell struct {
	ID        int                    `xml:"id,attr"`
	Memory    CapsHostNUMAMemory     `xml:"memory"`
	PageInfo  []CapsHostNUMAPageInfo `xml:"pages"`
	Distances []CapsHostNUMADistance `xml:"distances>sibling"`
	CPUS      []CapsHostNUMACPU      `xml:"cpus>cpu"`
}

type CapsHostNUMATopology struct {
	Cells []CapsHostNUMACell `xml:"cells>cell"`
}

type CapsHostSecModelLabel struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type CapsHostSecModel struct {
	Name   string                  `xml:"model"`
	DOI    string                  `xml:"doi"`
	Labels []CapsHostSecModelLabel `xml:"baselabel"`
}

type CapsHost struct {
	UUID     string                `xml:"uuid"`
	CPU      *CapsHostCPU          `xml:"cpu"`
	NUMA     *CapsHostNUMATopology `xml:"topology"`
	SecModel []CapsHostSecModel    `xml:"secmodel"`
}

type CapsGuestMachine struct {
	Name      string  `xml:",chardata"`
	MaxCPUs   int     `xml:"maxCpus,attr"`
	Canonical *string `xml:"canonical,attr"`
}

type CapsGuestDomain struct {
	Type     string             `xml:"type,attr"`
	Emulator string             `xml:"emulator"`
	Machines []CapsGuestMachine `xml:"machine"`
}

type CapsGuestArch struct {
	Name     string             `xml:"name,attr"`
	WordSize string             `xml:"wordsize"`
	Emulator string             `xml:"emulator"`
	Machines []CapsGuestMachine `xml:"machine"`
	Domains  []CapsGuestDomain  `xml:"domain"`
}

type CapsGuestFeatures struct {
	CPUSelection *struct{} `xml:"cpuselection"`
	DeviceBoot   *struct{} `xml:"deviceboot"`
}

type CapsGuest struct {
	OSType   string             `xml:"os_type"`
	Arch     CapsGuestArch      `xml:"arch"`
	Features *CapsGuestFeatures `xml:"features"`
}

type Caps struct {
	XMLName xml.Name    `xml:"capabilities"`
	Host    CapsHost    `xml:"host"`
	Guests  []CapsGuest `xml:"guest"`
}

func (c *Caps) Unmarshal(doc string) error {
	return xml.Unmarshal([]byte(doc), c)
}

func (c *Caps) Marshal() (string, error) {
	doc, err := xml.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(doc), nil
}
