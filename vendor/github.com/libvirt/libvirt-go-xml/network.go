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
 * Copyright (C) 2017 Lian Duan <blazeblue@gmail.com>
 *
 */

package libvirtxml

import (
	"encoding/xml"
)

type NetworkBridge struct {
	Name            string `xml:"name,attr,omitempty"`
	STP             string `xml:"stp,attr,omitempty"`
	Delay           string `xml:"delay,attr,omitempty"`
	MACTableManager string `xml:"macTableManager,attr,omitempty"`
}

type NetworkDomain struct {
	Name      string `xml:"name,attr,omitempty"`
	LocalOnly string `xml:"localOnly,attr,omitempty"`
}

type NetworkForward struct {
	Mode string `xml:"mode,attr,omitempty"`
	Dev  string `xml:"dev,attr,omitempty"`
}

type NetworkMAC struct {
	Address string `xml:"address,attr,omitempty"`
}

type NetworkDHCPRange struct {
	Start string `xml:"start,attr,omitempty"`
	End   string `xml:"end,attr,omitempty"`
}

type NetworkDHCPHost struct {
	ID   string `xml:"id,attr,omitempty"`
	MAC  string `xml:"mac,attr,omitempty"`
	Name string `xml:"name,attr,omitempty"`
	IP   string `xml:"ip,attr,omitempty"`
}

type NetworkDHCP struct {
	Ranges []NetworkDHCPRange `xml:"range"`
	Hosts  []NetworkDHCPHost  `xml:"host"`
}

type NetworkIP struct {
	Address  string       `xml:"address,attr,omitempty"`
	Family   string       `xml:"family,attr,omitempty"`
	Netmask  string       `xml:"netmask,attr,omitempty"`
	Prefix   string       `xml:"prefix,attr,omitempty"`
	LocalPtr string       `xml:"localPtr,attr,omitempty"`
	DHCP     *NetworkDHCP `xml:"dhcp"`
}

type NetworkRoute struct {
	Address string `xml:"address,attr,omitempty"`
	Family  string `xml:"family,attr,omitempty"`
	Prefix  string `xml:"prefix,attr,omitempty"`
	Metric  string `xml:"metric,attr,omitempty"`
	Gateway string `xml:"gateway,attr,omitempty"`
}

type Network struct {
	XMLName             xml.Name        `xml:"network"`
	IPv6                string          `xml:"ipv6,attr,omitempty"`
	TrustGuestRxFilters string          `xml:"trustGuestRxFilters,attr,omitempty"`
	Name                string          `xml:"name,omitempty"`
	UUID                string          `xml:"uuid,omitempty"`
	MAC                 *NetworkMAC     `xml:"mac"`
	Bridge              *NetworkBridge  `xml:"bridge"`
	Forward             *NetworkForward `xml:"forward"`
	Domain              *NetworkDomain  `xml:"domain"`
	IPs                 []NetworkIP     `xml:"ip"`
	Routes              []NetworkRoute  `xml:"route"`
}

func (s *Network) Unmarshal(doc string) error {
	return xml.Unmarshal([]byte(doc), s)
}

func (s *Network) Marshal() (string, error) {
	doc, err := xml.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(doc), nil
}
