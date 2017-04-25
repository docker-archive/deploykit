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

type StoragePoolSize struct {
	Unit  string `xml:"unit,attr,omitempty"`
	Value uint64 `xml:",chardata"`
}

type StoragePoolTargetPermissions struct {
	Owner string `xml:"owner,omitempty"`
	Group string `xml:"group,omitempty"`
	Mode  string `xml:"mode,omitempty"`
	Label string `xml:"label,omitempty"`
}

type StoragePoolTargetTimestamps struct {
	Atime string `xml:"atime"`
	Mtime string `xml:"mtime"`
	Ctime string `xml:"ctime"`
}

type StoragePoolTarget struct {
	Path        string                        `xml:"path,omitempty"`
	Permissions *StoragePoolTargetPermissions `xml:"permissions"`
	Timestamps  *StoragePoolTargetTimestamps  `xml:"timestamps"`
	Encryption  *StorageEncryption            `xml:"encryption"`
}

type StoragePoolSourceFormat struct {
	Type string `xml:"type,attr"`
}
type StoragePoolSourceHost struct {
	Name string `xml:"name,attr"`
}

type StoragePoolSourceDevice struct {
	Path          string `xml:"path,attr"`
	PartSeparator string `xml:"part_separator,attr,omitempty"`
}

type StoragePoolSourceAuthSecret struct {
	Usage string `xml:"usage,attr"`
}

type StoragePoolSourceAuth struct {
	Type     string                       `xml:"type,attr"`
	Username string                       `xml:"username,attr"`
	Secret   *StoragePoolSourceAuthSecret `xml:"secret"`
}

type StoragePoolSourceVendor struct {
	Name string `xml:"name,attr"`
}

type StoragePoolSourceProduct struct {
	Name string `xml:"name,attr"`
}

type StoragePoolSourceAdapterParentAddrAddress struct {
	Domain string `xml:"domain,attr"`
	Bus    string `xml:"bus,attr"`
	Slot   string `xml:"slot,attr"`
	Addr   string `xml:"addr,attr"`
}

type StoragePoolSourceAdapterParentAddr struct {
	UniqueID uint64                                     `xml:"unique_id,attr"`
	Address  *StoragePoolSourceAdapterParentAddrAddress `xml:"address"`
}

type StoragePoolSourceAdapter struct {
	Type       string                              `xml:"type,attr"`
	Name       string                              `xml:"name,attr,omitempty"`
	Parent     string                              `xml:"parent,attr,omitempty"`
	WWNN       string                              `xml:"wwnn,attr,omitempty"`
	WWPN       string                              `xml:"wwpn,attr,omitempty"`
	ParentAddr *StoragePoolSourceAdapterParentAddr `xml:"parentaddr"`
}

type StoragePoolSource struct {
	Host    *StoragePoolSourceHost    `xml:"host"`
	Device  *StoragePoolSourceDevice  `xml:"device"`
	Auth    *StoragePoolSourceAuth    `xml:"auth"`
	Vendor  *StoragePoolSourceVendor  `xml:"vendor"`
	Product *StoragePoolSourceProduct `xml:"product"`
	Format  *StoragePoolSourceFormat  `xml:"format"`
	Adapter *StoragePoolSourceAdapter `xml:"adapter"`
}

type StoragePool struct {
	XMLName    xml.Name           `xml:"pool"`
	Type       string             `xml:"type,attr"`
	Name       string             `xml:"name"`
	UUID       string             `xml:"uuid,omitempty"`
	Allocation *StoragePoolSize   `xml:"allocation,omitempty"`
	Capacity   *StoragePoolSize   `xml:"capacity,omitempty"`
	Available  *StoragePoolSize   `xml:"available,omitempty"`
	Target     *StoragePoolTarget `xml:"target"`
	Source     *StoragePoolSource `xml:"source"`
}

func (s *StoragePool) Unmarshal(doc string) error {
	return xml.Unmarshal([]byte(doc), s)
}

func (s *StoragePool) Marshal() (string, error) {
	doc, err := xml.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(doc), nil
}
