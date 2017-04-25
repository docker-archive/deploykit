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
 * Copyright (C) 2017 Red Hat, Inc.
 *
 */

package libvirtxml

import "encoding/xml"

type DomainSnapshotDiskDriver struct {
	Type string `xml:"type,attr"`
}

type DomainSnapshotDiskSource struct {
	File string `xml:"file,attr"`
}

type DomainSnapshotDisk struct {
	Name     string                    `xml:"name,attr"`
	Snapshot string                    `xml:"snapshot,attr,omitempty"`
	Driver   *DomainSnapshotDiskDriver `xml:"driver"`
	Source   *DomainSnapshotDiskSource `xml:"source"`
}

type DomainSnapshotDisks struct {
	Disks []DomainSnapshotDisk `xml:"disk"`
}

type DomainSnapshotMemory struct {
	Snapshot string `xml:"snapshot,attr"`
}

type DomainSnapshotParent struct {
	Name string `xml:"name"`
}

type DomainSnapshot struct {
	XMLName      xml.Name              `xml:"domainsnapshot"`
	Name         string                `xml:"name,omitempty"`
	Description  string                `xml:"description,omitempty"`
	State        string                `xml:"state,omitempty"`
	CreationTime string                `xml:"creationTime,omitempty"`
	Parent       *DomainSnapshotParent `xml:"parent"`
	Memory       *DomainSnapshotMemory `xml:"memory"`
	Disks        *DomainSnapshotDisks  `xml:"disks"`
	Domain       *Domain               `xml:"domain"`
}

func (s *DomainSnapshot) Unmarshal(doc string) error {
	return xml.Unmarshal([]byte(doc), s)
}

func (s *DomainSnapshot) Marshal() (string, error) {
	doc, err := xml.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(doc), nil
}
