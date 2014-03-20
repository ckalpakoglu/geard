/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dbus

import (
	"errors"
	"github.com/guelfey/go.dbus"
)

func (c *Conn) initJobs() {
	c.jobListener.jobs = make(map[dbus.ObjectPath]chan string)
}

func (c *Conn) jobComplete(signal *dbus.Signal) {
	var id uint32
	var job dbus.ObjectPath
	var unit string
	var result string
	dbus.Store(signal.Body, &id, &job, &unit, &result)
	c.jobListener.Lock()
	out, ok := c.jobListener.jobs[job]
	if ok {
		out <- result
	}
	c.jobListener.Unlock()
}

func (c *Conn) startJob(job string, args ...interface{}) (<-chan string, error) {
	c.jobListener.Lock()
	defer c.jobListener.Unlock()

	ch := make(chan string, 1)
	var path dbus.ObjectPath
	err := c.sysobj.Call(job, 0, args...).Store(&path)
	if err != nil {
		return nil, err
	}
	c.jobListener.jobs[path] = ch
	return ch, nil
}

func (c *Conn) runJob(job string, args ...interface{}) (string, error) {
	respCh, err := c.startJob(job, args...)
	if err != nil {
		return "", err
	}
	return <-respCh, nil
}

// LoadUnit is similar to GetUnit() but will load the unit from disk if
// possible.
func (c *Conn) LoadUnit(name string) (string, error) {
	var path dbus.ObjectPath
	err := c.sysobj.Call("LoadUnit", 0, name).Store(&path)
	if err != nil {
		return "", err
	}
	return string(path), nil
}

// StartUnit enqeues a start job and depending jobs, if any (unless otherwise
// specified by the mode string).
//
// Takes the unit to activate, plus a mode string. The mode needs to be one of
// replace, fail, isolate, ignore-dependencies, ignore-requirements. If
// "replace" the call will start the unit and its dependencies, possibly
// replacing already queued jobs that conflict with this. If "fail" the call
// will start the unit and its dependencies, but will fail if this would change
// an already queued job. If "isolate" the call will start the unit in question
// and terminate all units that aren't dependencies of it. If
// "ignore-dependencies" it will start a unit but ignore all its dependencies.
// If "ignore-requirements" it will start a unit but only ignore the
// requirement dependencies. It is not recommended to make use of the latter
// two options.
//
// Result string: one of done, canceled, timeout, failed, dependency, skipped.
// done indicates successful execution of a job. canceled indicates that a job
// has been canceled  before it finished execution. timeout indicates that the
// job timeout was reached. failed indicates that the job failed. dependency
// indicates that a job this job has been depending on failed and the job hence
// has been removed too. skipped indicates that a job was skipped because it
// didn't apply to the units current state.
func (c *Conn) StartUnit(name string, mode string) (string, error) {
	return c.runJob("StartUnit", name, mode)
}

func (c *Conn) StartUnitJob(name string, mode string) error {
	_, err := c.startJob("StartUnit", name, mode)
	return err
}

// StopUnit is similar to StartUnit but stops the specified unit rather
// than starting it.
func (c *Conn) StopUnit(name string, mode string) (string, error) {
	return c.runJob("StopUnit", name, mode)
}

func (c *Conn) StopUnitJob(name string, mode string) error {
	_, err := c.startJob("StopUnit", name, mode)
	return err
}

// ReloadUnit reloads a unit.  Reloading is done only if the unit is already running and fails otherwise.
func (c *Conn) ReloadUnit(name string, mode string) (string, error) {
	return c.runJob("ReloadUnit", name, mode)
}

// RestartUnit restarts a service.  If a service is restarted that isn't
// running it will be started.
func (c *Conn) RestartUnit(name string, mode string) (string, error) {
	return c.runJob("RestartUnit", name, mode)
}

func (c *Conn) RestartUnitJob(name string, mode string) error {
	_, err := c.startJob("RestartUnit", name, mode)
	return err
}

// TryRestartUnit is like RestartUnit, except that a service that isn't running
// is not affected by the restart.
func (c *Conn) TryRestartUnit(name string, mode string) (string, error) {
	return c.runJob("TryRestartUnit", name, mode)
}

// ReloadOrRestart attempts a reload if the unit supports it and use a restart
// otherwise.
func (c *Conn) ReloadOrRestartUnit(name string, mode string) (string, error) {
	return c.runJob("ReloadOrRestartUnit", name, mode)
}

// ReloadOrTryRestart attempts a reload if the unit supports it and use a "Try"
// flavored restart otherwise.
func (c *Conn) ReloadOrTryRestartUnit(name string, mode string) (string, error) {
	return c.runJob("ReloadOrTryRestartUnit", name, mode)
}

// StartTransientUnit() may be used to create and start a transient unit, which
// will be released as soon as it is not running or referenced anymore or the
// system is rebooted. name is the unit name including suffix, and must be
// unique. mode is the same as in StartUnit(), properties contains properties
// of the unit.
func (c *Conn) StartTransientUnit(name string, mode string, properties ...Property) (string, error) {
	// the dbus interface for this method does not use the last argument and
	// should simply be given an empty list.  We use a concrete type here
	// (instead of the more appropriate interface{}) to satisfy the dbus library.
	return c.runJob("StartTransientUnit", name, mode, properties, make([]string, 0))
}

// KillUnit takes the unit name and a UNIX signal number to send.  All of the unit's
// processes are killed.
func (c *Conn) KillUnit(name string, signal int32) {
	c.sysobj.Call("KillUnit", 0, name, "all", signal).Store()
}

// GetUnitProperties takes the unit name and returns all of its dbus object properties.
func (c *Conn) GetUnitProperties(unit string) (map[string]interface{}, error) {
	var err error
	var props map[string]dbus.Variant

	path := ObjectPath("/org/freedesktop/systemd1/unit/" + unit)
	if !path.IsValid() {
		return nil, errors.New("invalid unit name: " + unit)
	}

	obj := c.sysconn.Object("org.freedesktop.systemd1", path)
	err = obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, "org.freedesktop.systemd1.Unit").Store(&props)
	if err != nil {
		return nil, err
	}

	out := make(map[string]interface{}, len(props))
	for k, v := range props {
		out[k] = v.Value()
	}

	return out, nil
}

// SetUnitProperties() may be used to modify certain unit properties at runtime.
// Not all properties may be changed at runtime, but many resource management
// settings (primarily those in systemd.cgroup(5)) may. The changes are applied
// instantly, and stored on disk for future boots, unless runtime is true, in which
// case the settings only apply until the next reboot. name is the name of the unit
// to modify. properties are the settings to set, encoded as an array of property
// name and value pairs.
func (c *Conn) SetUnitProperties(name string, runtime bool, properties ...Property) error {
	return c.sysobj.Call("SetUnitProperties", 0, name, runtime, properties).Store()
}

// ListUnits returns an array with all currently loaded units. Note that
// units may be known by multiple names at the same time, and hence there might
// be more unit names loaded than actual units behind them.
func (c *Conn) ListUnits() ([]UnitStatus, error) {
	result := make([][]interface{}, 0)
	err := c.sysobj.Call("ListUnits", 0).Store(&result)
	if err != nil {
		return nil, err
	}

	resultInterface := make([]interface{}, len(result))
	for i := range result {
		resultInterface[i] = result[i]
	}

	status := make([]UnitStatus, len(result))
	statusInterface := make([]interface{}, len(status))
	for i := range status {
		statusInterface[i] = &status[i]
	}

	err = dbus.Store(resultInterface, statusInterface...)
	if err != nil {
		return nil, err
	}

	return status, nil
}

type UnitStatus struct {
	Name        string          // The primary unit name as string
	Description string          // The human readable description string
	LoadState   string          // The load state (i.e. whether the unit file has been loaded successfully)
	ActiveState string          // The active state (i.e. whether the unit is currently started or not)
	SubState    string          // The sub state (a more fine-grained version of the active state that is specific to the unit type, which the active state is not)
	Followed    string          // A unit that is being followed in its state by this unit, if there is any, otherwise the empty string.
	Path        dbus.ObjectPath // The unit object path
	JobId       uint32          // If there is a job queued for the job unit the numeric job id, 0 otherwise
	JobType     string          // The job type as string
	JobPath     dbus.ObjectPath // The job object path
}

// EnableUnitFiles() may be used to enable one or more units in the system (by
// creating symlinks to them in /etc or /run).
//
// It takes a list of unit files to enable (either just file names or full
// absolute paths if the unit files are residing outside the usual unit
// search paths), and two booleans: the first controls whether the unit shall
// be enabled for runtime only (true, /run), or persistently (false, /etc).
// The second one controls whether symlinks pointing to other units shall
// be replaced if necessary.
//
// This call returns one boolean and an array with the changes made. The
// boolean signals whether the unit files contained any enablement
// information (i.e. an [Install]) section. The changes list consists of
// structures with three strings: the type of the change (one of symlink
// or unlink), the file name of the symlink and the destination of the
// symlink.
func (c *Conn) EnableUnitFiles(files []string, runtime bool, force bool) (bool, []EnableUnitFileChange, error) {
	var carries_install_info bool

	result := make([][]interface{}, 0)
	err := c.sysobj.Call("EnableUnitFiles", 0, files, runtime, force).Store(&carries_install_info, &result)
	if err != nil {
		return false, nil, err
	}

	resultInterface := make([]interface{}, len(result))
	for i := range result {
		resultInterface[i] = result[i]
	}

	changes := make([]EnableUnitFileChange, len(result))
	changesInterface := make([]interface{}, len(changes))
	for i := range changes {
		changesInterface[i] = &changes[i]
	}

	err = dbus.Store(resultInterface, changesInterface...)
	if err != nil {
		return false, nil, err
	}

	return carries_install_info, changes, nil
}

type EnableUnitFileChange struct {
	Type        string // Type of the change (one of symlink or unlink)
	Filename    string // File name of the symlink
	Destination string // Destination of the symlink
}

// DisableUnitFiles() may be used to disable one or more units in the system (by
// removing symlinks to them from /etc or /run).
//
// It takes a list of unit files to disable (either just file names or full
// absolute paths if the unit files are residing outside the usual unit
// search paths), and one boolean: whether the unit was enabled for runtime
// only (true, /run), or persistently (false, /etc).
//
// This call returns an array with the changes made. The changes list
// consists of structures with three strings: the type of the change (one of
// symlink or unlink), the file name of the symlink and the destination of the
// symlink.
func (c *Conn) DisableUnitFiles(files []string, runtime bool) ([]DisableUnitFileChange, error) {
	result := make([][]interface{}, 0)
	err := c.sysobj.Call("DisableUnitFiles", 0, files, runtime).Store(&result)
	if err != nil {
		return nil, err
	}

	resultInterface := make([]interface{}, len(result))
	for i := range result {
		resultInterface[i] = result[i]
	}

	changes := make([]DisableUnitFileChange, len(result))
	changesInterface := make([]interface{}, len(changes))
	for i := range changes {
		changesInterface[i] = &changes[i]
	}

	err = dbus.Store(resultInterface, changesInterface...)
	if err != nil {
		return nil, err
	}

	return changes, nil
}

type DisableUnitFileChange struct {
	Type        string // Type of the change (one of symlink or unlink)
	Filename    string // File name of the symlink
	Destination string // Destination of the symlink
}

// Reload instructs systemd to scan for and reload unit files. This is
// equivalent to a 'systemctl daemon-reload'.
func (c *Conn) Reload() error {
	return c.sysobj.Call("Reload", 0).Store()
}