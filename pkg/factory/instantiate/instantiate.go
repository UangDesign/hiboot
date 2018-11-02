// Copyright 2018 John Deng (hi.devops.io@gmail.com).
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package instantiate implement InstantiateFactory
package instantiate

import (
	"errors"
	"fmt"
	"hidevops.io/hiboot/pkg/factory"
	"hidevops.io/hiboot/pkg/factory/depends"
	"hidevops.io/hiboot/pkg/inject"
	"hidevops.io/hiboot/pkg/log"
	"hidevops.io/hiboot/pkg/system/types"
	"hidevops.io/hiboot/pkg/utils/cmap"
	"hidevops.io/hiboot/pkg/utils/reflector"
)

var (
	// ErrNotInitialized InstantiateFactory is not initialized
	ErrNotInitialized = errors.New("[factory] InstantiateFactory is not initialized")

	// ErrInvalidObjectType invalid object type
	ErrInvalidObjectType = errors.New("[factory] invalid object type")
)

// InstantiateFactory is the factory that responsible for object instantiation
type instantiateFactory struct {
	instanceMap      cmap.ConcurrentMap
	components       []*factory.MetaData
	categorized      map[string][]interface{}
	customProperties cmap.ConcurrentMap
}

// NewInstantiateFactory the constructor of instantiateFactory
func NewInstantiateFactory(instanceMap cmap.ConcurrentMap, components []*factory.MetaData, customProperties cmap.ConcurrentMap) factory.InstantiateFactory {
	return &instantiateFactory{
		instanceMap:      instanceMap,
		components:       components,
		categorized:      make(map[string][]interface{}),
		customProperties: customProperties,
	}
}

// Initialized check if factory is initialized
func (f *instantiateFactory) Initialized() bool {
	return f.instanceMap != nil
}

// AppendComponent append component
func (f *instantiateFactory) AppendComponent(c ...interface{}) {
	metaData := factory.NewMetaData(c...)
	f.components = append(f.components, metaData)
}

// BuildComponents build all registered components
func (f *instantiateFactory) BuildComponents() (err error) {
	// first resolve the dependency graph
	var resolved []*factory.MetaData
	log.Infof("Resolving dependencies")
	resolved, err = depends.Resolve(f.components)
	log.Infof("Injecting dependencies")
	// then build components
	var obj interface{}
	var name string
	for i, item := range resolved {
		// inject dependencies into function
		// components, controllers
		switch item.Kind {
		case types.Func:
			obj, err = inject.IntoFunc(item.Object)
			name = item.Name
			// TODO: should report error when err is not nil
			if err == nil {
				log.Debugf("%d: inject into func: %v %v", i, item.ShortName, item.Type)
			}
		case types.Method:
			obj, err = inject.IntoMethod(item.Context, item.Object)
			name = item.Name
			if err == nil {
				log.Debugf("%d: inject into method: %v %v", i, item.ShortName, item.Type)
			}
		default:
			name, obj = item.Name, item.Object
		}
		if obj != nil {
			// inject into object
			err = inject.IntoObject(obj)
			tagName, ok := reflector.FindEmbeddedFieldTag(obj, "Qualifier", "name")
			if ok {
				name = tagName
				log.Debugf("name: %v, Qualifier: %v, ok: %v", item.Name, name, ok)
			}

			if name != "" {
				err = f.SetInstance(name, obj)
			}
		}
	}
	if err == nil {
		log.Infof("Injected dependencies")
	}
	return
}

// SetInstance save instance
func (f *instantiateFactory) SetInstance(params ...interface{}) (err error) {
	if !f.Initialized() {
		return ErrNotInitialized
	}

	name, instance := factory.ParseParams(params...)

	if _, ok := f.instanceMap.Get(name); ok {
		return fmt.Errorf("instance name %v is already taken", name)
	}

	f.instanceMap.Set(name, instance)

	ifcField := reflector.GetEmbeddedField(instance, "")
	if ifcField.Anonymous {
		typeName := ifcField.Name
		categorised, ok := f.categorized[typeName]
		if !ok {
			categorised = make([]interface{}, 0)
		}
		f.categorized[typeName] = append(categorised, instance)
	}
	return
}

// GetInstance get instance by name
func (f *instantiateFactory) GetInstance(params ...interface{}) (retVal interface{}) {
	if !f.Initialized() {
		return nil
	}

	name, _ := factory.ParseParams(params...)

	//items := f.Items()
	//log.Debug(items)

	var ok bool
	if retVal, ok = f.instanceMap.Get(name); !ok {
		return nil
	}
	return
}

// GetInstances get instance by name
func (f *instantiateFactory) GetInstances(name string) (retVal []interface{}) {
	//items := f.Items()
	//log.Debug(items)
	if f.Initialized() {
		retVal = f.categorized[name]
	}
	return
}

// Items return instance map
func (f *instantiateFactory) Items() map[string]interface{} {
	if !f.Initialized() {
		return nil
	}
	return f.instanceMap.Items()
}

// Items return instance map
func (f *instantiateFactory) CustomProperties() map[string]interface{} {
	return f.customProperties.Items()
}
