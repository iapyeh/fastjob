// version: 2019-09-19T15:41:05+00:00
// required objshpb.js

//in global namespace
function GetSDKSingleton(){
    //Call this to create ObjshSDK()
    if (ObjshSDK.singleton) return ObjshSDK.singleton
    ObjshSDK.singleton = new ObjshSDK()
    return ObjshSDK.singleton
}
function ObjshSDK(){
    this.listeners = {}
    this.user = new ObjshSDK.User(this)
}
ObjshSDK.singleton = null

/* utilities*/
ObjshSDK.Utility = {
    get_header:function(url,callback){
        var xmlhttp = new XMLHttpRequest();
        http.open('HEAD', url);
        http.onreadystatechange = function() {
            if (xmlhttp.readyState==2){
                var header_lines = xmlhttp.getAllResponseHeaders().split('\n')
                var headers = {}
                for (var i=0;i<header_lines.length;i++){
                    var p = line.indexOf(':')
                    headers[line.substring(0,p).trim()] = line.substring(p+1).trim()
                }
                callback(this.status,headers)
            }            
        };
        http.send();
    }
    ,request:function(url,method,parameters,callback,is_blob,progress_callback){
        if (method=='GET')ObjshSDK.Utility.http_get(url,parameters,callback,is_blob,progress_callback)
        else if (method=='POST')ObjshSDK.Utility.http_post(url,parameters,callback,is_blob,progress_callback)
    }
    ,http_get:function(url,parameters,callback,is_blob,progress_callback){
       ObjshSDK.Utility.get_ajax().get(url, parameters, callback, is_blob,progress_callback)
    }
    ,http_post:function(url,parameters,callback,is_blob,progress_callback){
        ObjshSDK.Utility.get_ajax().post(url, parameters, callback,is_blob,progress_callback)
    }
    ,get_ajax:function(){
        if (ObjshSDK.Utility.ajax) return ObjshSDK.Utility.ajax
        var ajax = {};
        ajax.x = function () {
            if (typeof XMLHttpRequest !== 'undefined') {
                return new XMLHttpRequest();
            }
            var versions = [
                "MSXML2.XmlHttp.6.0",
                "MSXML2.XmlHttp.5.0",
                "MSXML2.XmlHttp.4.0",
                "MSXML2.XmlHttp.3.0",
                "MSXML2.XmlHttp.2.0",
                "Microsoft.XmlHttp"
            ];
        
            var xhr;
            for (var i = 0; i < versions.length; i++) {
                try {
                    xhr = new ActiveXObject(versions[i]);
                    break;
                } catch (e) {
                }
            }
            return xhr;
        };
        
        ajax.send = function (url, callback, method, data, is_blob, progress_callback, async) {
            if (async === undefined) {
                async = true;
            }
            var x = ajax.x();
            
            if (is_blob) x.responseType = "blob";
            x.onprogress = function(oEvent){
                if (oEvent.lengthComputable) {
                    var percentComplete = oEvent.loaded / oEvent.total;
                    if (progress_callback) progress_callback(percentComplete)
                } else {
                    if (progress_callback) progress_callback('...')
                }            
            };

            x.open(method, url, async);
            x.onreadystatechange = function () {
                if (x.readyState == 4) {
                    if (callback) callback(x.responseText)
                }
            };
            if (method == 'POST') {
                x.setRequestHeader('Content-type', 'application/x-www-form-urlencoded');
            }
            x.send(data)
        };
        
        ajax.get = function (url, data, callback, is_blob, progress_callback, async) {
            var query = [];
            for (var key in data) {
                query.push(encodeURIComponent(key) + '=' + encodeURIComponent(data[key]));
            }
            ajax.send(url + (query.length ? '?' + query.join('&') : ''), callback, 'GET', null, is_blob, progress_callback, async)
        };
        
        ajax.post = function (url, data, callback, is_blob,progress_callback, async) {
            var query = [];
            for (var key in data) {
                query.push(encodeURIComponent(key) + '=' + encodeURIComponent(data[key]));
            }
            ajax.send(url, callback, 'POST', query.join('&'),is_blob,progress_callback, async)
        };
        ObjshSDK.Utility.ajax = ajax
        return ajax
    }
}

ObjshSDK.prototype ={
    makeDeferred: function(){
        return new ObjshSDK.Deferred()
    }
    ,login:function(options){
        /*
        options:{
            url: (string) path to request login, default to /login
            usernamae: (string)
            password:(string)
            method:(string) GET or POST when sending request
        } 
        Return: a promise
        resolve: ObjshSDK.User instance
        reject: null if failed to login
        */
        if (typeof options == 'undefined') options = {}
        var url
        if (options.url) url = options.url
        else url = '/login'
        var method = options.method ? options.method.toUpperCase() : 'GET'
        var parameters = {
            username:options.username || '',
            password:options.password || ''
        }
        var self = this
        var promise = new ObjshSDK.Deferred()
        ObjshSDK.Utility.request(url,method,parameters,function(response){
            var userdata = JSON.parse(response)
            if (userdata.username) {
                // login succeeded
                self.user.setUserdata(userdata)
                promise.resolve(self.user)
            }
            else promise.reject(null)
        })
        return promise
    }
    ,logout:function(urlPath){
        if (!urlPath) urlPath = '/logout?oknext='+encodeURIComponent(location.pathname)
        var self = this
        var promise = new ObjshSDK.Deferred()
        ObjshSDK.Utility.http_get(urlPath,{},function(response){
            self.user = null
            delete self.user
            promise.resolve()
        })
        return promise
    }
    //Event System
    ,on:function(name,callback){
        if (typeof(this.listeners[name]=='undefined')) this.listeners[name] = []
        this.listeners[name].push(callback)
    }
    ,fire:function(name,payload){
        if (typeof(this.listeners[name])=='undefined') return
        this.listeners[name].forEach(function(callback){
            callback(payload)
        })
    }
    //Commnunication System, default to protocol buffer
    ,useTree:function(treeName,url){
        if (!url) {
            url =  (location.protocol == "https:" ? "wss" : "ws") + "://"+location.host+"/objsh/pri/tree"
        }else if (url.indexOf('ws') != 0){
            url =  (location.protocol == "https:" ? "wss" : "ws") + "://"+location.host+url
        }
        this.tree = new ObjshSDK.Tree(this,url,treeName)
        var promise = new ObjshSDK.Deferred()
        if (url){
            var self = this
            var origin_onopen = this.tree.onopen
            this.tree.onopen = function(){
                promise.resolve(self.tree)
                origin_onopen()
            }
            this.tree.onerror = function(){
                promise.resolve(self.tree)
            }
        }
        else {
            promise.resolve(this.tree)
        }
        return promise
    }
    ,call:function(){
        if (!this.tree) return console.warn('call sdk without tree')
        return this.tree.call.apply(this.tree,arguments)
    }
}

ObjshSDK.Protobuf = function(packageName){
    this.packageName = packageName
    if (!proto[this.packageName]){
        console.error(this.packageName + " is not found in namespace of protocol buffer")
    }
    this.onopen = this.onclose = this.onerror = this.onmessage = function(){}
    this.ws = null
    this.listeners = {}
    //if true, don't call _set_obj automatically
    this.lazy = false 
    
    // probe all available message type of protocol buffer messages
    // 目前找不到可靠的方法知道message type
    // guess the typeUrl of given messge
    this.pbTypes = {}
    for (var key in proto) {
        if (key=='google' || key==undefined) continue
        else if (key == 'Any'){
            //exclude "Any"
            //this.pbTypes['Any'] = proto[key]
        }else{
            for (var prop in proto[key]){
                var typeName = key+'.'+prop
                this.pbTypes[typeName] = proto[key][prop]
            }
        }
    }
}
ObjshSDK.Protobuf.prototype = {
    connect:function(url){
        var self = this
        this.ws = new WebSocket(url)
        this.ws.binaryType = "arraybuffer"
        this.ws.onopen = function(evt){self.onopen(evt)}
        this.ws.onerror = function(evt){self.onerror(evt)}
        this.ws.onclose = function(evt){self.onclose(evt)}
        
        this.ws.onmessage = function(evt){
            var binary = new Uint8Array(evt.data)
            var pbAny = proto.google.protobuf.Any.deserializeBinary(binary)
            var typeName = pbAny.getTypeUrl().split('.').pop()
            var pb = proto[self.packageName][typeName].deserializeBinary(pbAny.getValue())
            self.onmessage(self.message(typeName,undefined,pb))
        }
    }
    ,message:function(typeName,initDict,initValue){
        return new ObjshSDK.ProtobufMessage(this,typeName,initDict,initValue,this.lazy)
    }
}
ObjshSDK.ProtobufMessage = function(protobuf,typeName,initDict,initValue,lazy){
    var self = this
    this.typeName = typeName
    this.protobuf = protobuf
    this.lazy = typeof(lazy) == 'undefined' ? false : lazy
    if (typeof(initValue) != 'undefined'){
        //mostly called by response from server
        this.value = initValue
        if (!this.lazy) this._set_obj()
    }
    else if (proto[this.protobuf.packageName][typeName]){
        //mostly called by browser to make a new message
        this.value = new proto[this.protobuf.packageName][typeName]()
        if (typeof(initDict) != 'undefined') this.update(initDict)
    }
    else console.error(typeName+' is not found in '+this.protobuf.packageName) 
}
ObjshSDK.ProtobufMessage.NewAnyMessage = function(protobuf, bareMessage){
    //bareMessage is an instance of proto.Message 
    if (typeof bareMessage.serializeBinary != 'function'){
        return null //this is not an instance of proto.Message
    }
    for (var typeName in protobuf.pbTypes){
        if (protobuf.pbTypes[typeName] !== bareMessage.constructor) continue
        var anyMsg = new proto.google.protobuf.Any()
        // 必須多加一個倒斜線成為 "/objsh.Command"，
        // 因為Golang的ptypes.AnyMessageName需要它
        var typeUrl = typeName
        anyMsg.setTypeUrl((typeUrl.indexOf('/')==-1 ? "/" : "") + typeUrl)
        anyMsg.setValue(bareMessage.serializeBinary())
        return anyMsg
    }
    return null
}
ObjshSDK.ProtobufMessage.prototype = {
    _set_obj:function(){
        this._obj = proto[this.protobuf.packageName][this.typeName].toObject(false,this.value)
        for (var key in this._obj){
            var Name = (key.charAt(0).toUpperCase() + key.slice(1).toLowerCase())
            if (this.value['get'+Name+'_asU8']){
                //this is a byte array value, let convert it to string
                this._obj[key] = new TextDecoder("utf-8").decode(this.value['get'+Name+'_asU8']());
            }
        }

    }
    ,update:function(aDict){
        var self = this
        for (var key in aDict){
            var Name = (key.charAt(0).toUpperCase() + key.slice(1).toLowerCase())
            var sname =  'set'+ Name
            if (this.value[sname]){
                this.value[sname](aDict[key])
                continue
            }
            var aname = 'add'+Name
            if (this.value[aname]) {
                this.value['clear'+Name+'List']()//clear list
                var fn = this.value[aname]
                aDict[key].forEach(function(item){fn.call(self.value,item)})
                continue
            }
            var mname = 'get'+Name+'Map'
            if (this.value[mname]) {
                var map = this.value[mname]()
                map.clear() //clear map
                for (var k in aDict[key]){
                    map.set(k,aDict[key][k])
                }
                continue
            }
        }
        if (!this.lazy) this._set_obj()
    }
    ,emit:function(){
        var anyCmd = new proto.google.protobuf.Any()
        // 必須多加一個倒斜線成為 "/objsh.Command"，
        // 因為Golang的ptypes.AnyMessageName需要它
        var typeUrl = this.protobuf.packageName+"."+this.typeName
        anyCmd.setTypeUrl((typeUrl.indexOf('/')==-1 ? "/" : "") + typeUrl)
        anyCmd.setValue(this.value.serializeBinary())
        this.protobuf.ws.send(anyCmd.serializeBinary())
    }
    ,get:function(name){
        
        if (!this._obj) this._set_obj()

        if (typeof(this._obj[name]) != 'undefined') return this._obj[name]
        else if (typeof(this._obj[name+'List']) != 'undefined') return this._obj[name+'List']
        else if (typeof(this._obj[name+'Map']) != 'undefined') {
            var obj = {}
            this._obj[name+'Map'].forEach(function(kv){
                obj[kv[0]] = kv[1]
            })
            return obj
        }
    }
    ,toObject:function(){
        var obj = {}
        for (var name in this._obj){
            if (/List$/.test(name)) obj[name.substr(0,name.length-4)] = this._obj[name]
            else if (/Map$/.test(name)) {
                var mapobj = {}
                //map value in _obj is paired like [[k1,v1],[k2,v2]]
                this._obj[name].forEach(function(kv){
                    mapobj[kv[0]] = kv[1]
                })
                obj[name.substr(0,name.length-3)] = mapobj
            }
            else obj[name] = this._obj[name]
        }
        return obj
    }
    ,set:function(name,value){
        var self = this
        var Name = (name.charAt(0).toUpperCase() + name.slice(1).toLowerCase())
        if (typeof(this._obj[name]) != 'undefined') {
            this._obj[name]  = value
            this.value['set'+Name](value)
        }
        else if (typeof(this._obj[name+'List']) != 'undefined'){
            this._obj[name+'List'] = value
            this.value['clear'+Name+'List']()
            var fn = this.value['add'+Name]
            value.forEach(function(item){fn.call(self.value,item)})
        }
        else if (typeof(this._obj[name+'Map']) != 'undefined') {
            var map = this.value['get'+Name+'Map']()
            map.clear()
            this._obj[name+'Map'] = []
            for (var k in value){
                map.set(k,value[k])
                this._obj[name+'Map'].push([k,value[k]])
            }
        }
    }    
}

ObjshSDK.Deferred = function(){
    this.progressListener = []
    this.doneListener = []
    this.failListener = []
    this.thenListener = [] //notify and done
    this.killer = null
    this.background = false //flag of background task
    this.resolved = undefined
}
ObjshSDK.Deferred.prototype = {
    progress:function(callback){
        this.progressListener.push(callback)
        return this
    }
    ,done: function(callback){
        if (this.resolved != undefined){
            this.fire([callback],this.resolved)
            return this
        }
        this.doneListener.push(callback)
        return this
    }
    ,then: function(callback){
        this.thenListener.push(callback)
        return this
    }
    ,always:function(callback){
        //alias of then
        return this.then(callback)
    }
    ,fail: function(callback){
        this.failListener.push(callback)
        return this
    }  
    ,notify:function(){
        this.fire(this.thenListener, arguments)
        this.fire(this.progressListener, arguments)
    }
    ,resolve: function(){
        this.resolved = arguments
        this.fire(this.doneListener, arguments)
        this.fire(this.thenListener, arguments)
    }
    ,reject: function(){
        this.fire(this.failListener, arguments)
        this.fire(this.thenListener, arguments)
    } 
    ,fire: function(listeners, args){
        var i = listeners.length
        args = Array.prototype.slice.call(args)
        while(i--) listeners[i].apply(null, args)
    }
    ,kill:function(){
        if (this.killer) return this.killer()
    }
}
  
ObjshSDK.Tree = function (sdk, url,treeName,packageName){
    // default treeName is "", which means caller has to specifiy full name
    this.sdk = sdk
    this.treeName = (typeof(treeName)=='undefined' ? '' : treeName)
    this.onopen = function(){sdk.fire('tree:open')}
    this.onclose = function(){sdk.fire('tree:close')}
    this.onerror = this.onmessage =  function(){}
    this.onannouce = function(content){console.log('ANNOUNCE: '+JSON.stringify(content))}
    // default packageName is "objsh",
    // change packageName to your own let Tree act as a gPRC-like style
    this.protobuf = new ObjshSDK.Protobuf((typeof(packageName)=='undefined' ? 'objsh' : packageName))
    this.protobuf.lazy = true
    this.nodes = {}
    this.queue = {}
    this.utf8Decoder = new TextDecoder("utf-8")
    if (url) this.connect(url)
}
ObjshSDK.Tree.prototype = {
    connect:function(url){
        var self = this
        //initailly, request tree's layout
        //url += (url.indexOf('?') == -1 ? '?' : '&') + 'layout=0' 
        this.protobuf.connect(url)
        this.protobuf.onopen=function(e){self.onopen(e)}
        this.protobuf.onclose=function(e){self.onclose(e)}
        this.protobuf.onerror=function(e){self.onerror(e)}
        this.protobuf.onmessage = function(message){
            var id = message.value.getId()
            /*
            if (id==0){
                //layout
                var layout = self.utf8Decoder.decode(message.value.getStdout_asU8());
                self.onannouce(layout)
                return
            }*/
            var data = null;
            try{
                data = self.queue[id]
            }catch(e){
                console.log('err=',e)
                return
            }
            if (data){
                switch(message.value.getRetcode()){
                    case 0:
                        //success result
                        var stdout = self.utf8Decoder.decode(message.value.getStdout_asU8());
                        var stdoutObj = '';
                        try{
                            stdoutObj = (stdout == '') ? '' : JSON.parse(stdout)
                        }catch(e){
                            console.log(stdout)
                            console.warn(e)
                        }
                        data.deferred.resolve(stdoutObj)
                        delete self.queue[id]
                        break
                    case -2:
                        //progress result from background task
                        data.deferred.background = true
                    case -1:
                        // progress result
                        var stdout = self.utf8Decoder.decode(message.value.getStdout_asU8());
                        data.deferred.notify(JSON.parse(stdout))
                        break
                    default:
                        // error result; suppose result is string
                        //var stderr = message.value['getStderr']();
                        //data.deferred.reject({code:message.value.getRetcode(),message:stderr})
                        data.deferred.reject(message.value.getRetcode(),message.value['getStderr']())
                        delete self.queue[id]
                }
            }
            else{
                var stdout = self.utf8Decoder.decode(message.value.getStdout_asU8());
                try{
                    self.onannouce(JSON.parse(stdout))
                }catch(e){
                    console.log(e)
                    console.log('[stdout]=',[stdout])
                    self.onannouce(stdout)
                }
                
            }
        }
    }
    ,call:function(branchName){
        //Support call styles:
        // call(branchName,[arg])
        // call(branchName,proto.Message)
        // call(branchName,{k:v})
        // call(branchName,[arg],{k:v})
        // call(branchName,{k:v},proto.Message)
        // call(branchName,[arg],proto.Message)
        // call(branchName,[arg],{k:v},proto.Message)
        // find last non-undefined argument
        var self = this
        var tailIdx = arguments.length //exclusive
        for (var i=tailIdx-1;i>=0;i--){
            if (typeof arguments[i] != 'undefined' && arguments[i] !== null) break
            tailIdx -= 1
        }
        if (tailIdx == 0) return //kidding
        var args = null
        var kw = null
        var pbMsg = null
        if (tailIdx > 1){
            //check  args , kw and pbMsg
            for (var i=1;i<tailIdx;i++){
                var arg = arguments[i]
                if (Array.isArray(arg)) args = arg
                else if (arg === Object(arg)) {
                    pbMsg = ObjshSDK.ProtobufMessage.NewAnyMessage(this.protobuf,arg)
                    //pbMsg = arg
                    if (!pbMsg) kw = arg
                }
                else {
                    console.warn('.call('+branchName+') ignore argument:'+arg)
                }
            }
            /*
            console.log('argum=ents=',arguments)
            console.log('branchName=',branchName)
            console.log('pbArg',pbMsg)
            console.log('args=',args)
            console.log('kw=',kw)
            */
        }
        var data = {
            //message id, for receiving response
            id: (new String(Math.floor(1000 * (new Date().getTime()+Math.random()))).substr(2)) % 2147483648,
            name: (this.treeName=='' ? '' : this.treeName+'.') +branchName,
        }
        if (args && args.length) {
            //Converting to string to compatible with pb.Command's spec
            data.args = []
            args.forEach(function(arg){
                data.args.push(new String(arg))
            })
        }
        if (kw){
            //Converting to string to compatible with pb.Command's spec
            data.kw = {}
            for (var k in kw){
                data.kw[new String(k)] = new String(kw[k])
            }
        }
        if (pbMsg){
           data.message = pbMsg
        }
        var command = this.protobuf.message('Command',data)
        command.emit()
        
        var deferred = new ObjshSDK.Deferred()
        deferred.killer = function(){
            return self.kill(this.c)
        }.bind({c:data})
        
        this.queue[data.id] = {deferred:deferred}
        return deferred
    }
    ,hook:function(cmdID,cmdPath){
        //A handy function to call hook and watch in a background task
        if (typeof cmdPath == 'undefined') cmdPath = '$.Hook' //default to $.Hook
        var deferred = new ObjshSDK.Deferred()
        
        if (this.queue[cmdID]){
            //double hook , or hook to a task in the same browser session
            setTimeout(function(){
                deferred.reject({code:403,message:"duplicated command ID"})
            })
            return deferred
        }

        //wrap the watch's deferred to my defer's output
        var watchtask = this.watch(cmdID)
        watchtask.done(function(stdout){
            deferred.done(stdout)
        }).progress(function(stdout){
            deferred.notify(stdout)
        }).fail(function(err){
            deferred.reject(err)
        })
    
        this.call(cmdPath,[cmdID]).done(function(stdout){

        }).fail(function(err){
            deferred.reject(err)
            watchtask.kill()
        })

        deferred.killer = watchtask.killer
        
        return deferred
    }
    ,watch:function(cmdID,cmdPath){
        if (typeof cmdPath == 'undefined') cmdPath = '$.Unhook' //default to $.Unhook
        var deferred = new ObjshSDK.Deferred()
        if (this.queue[cmdID]){
            setTimeout(function(){
                deferred.reject({code:403,message:"duplicated command ID"})
            })
            return deferred
        }        
        var self = this
        deferred.killer = function(){
            var cmdID = this.cmdID
            delete self.queue[cmdID]
            sdk.tree.call(cmdPath,[cmdID]).fail(function(err){
                sdk.tree.onannouce('tree.watch.kill: '+JSON.stringify(err))
            })
        }.bind({cmdID:cmdID})
        this.queue[cmdID] = {deferred:deferred}
        return deferred
    }
    ,kill:function(command_data){
        var branch_name = command_data.name
        // 2019-11-21T13:10:14+00:00 
        // why?
        //var name = (this.treeName=='' ? '' : this.treeName+'.') +branch_name

        var name = branch_name
        // 2019-11-22T04:20:58+00:00
        //  本來kill command的ID就是被kill command的ID，但這樣原來的command收不到reject的訊息
        //  因此改成不一樣的ID
        var idToKill = command_data.id
        var cmdId = (new String(Math.floor(1000 * (new Date().getTime()+Math.random()))).substr(2)) % 2147483648
        var command = this.protobuf.message('Command',{
            id: cmdId
            ,name: new String(idToKill)
            ,kill:true
        })
        command.emit()        
       var deferred = new ObjshSDK.Deferred()
       this.queue[cmdId] = {deferred:deferred,id:cmdId,name:name}
       return deferred
    }
}


//User
ObjshSDK.User = function (sdk,userdata){
    this.sdk = sdk
    this.preferences = new ObjshSDK.Preferences(sdk)
    //called when user has synced preferences from server
    this.promise = new ObjshSDK.Deferred()
    this.promise.resolve(true)

    // default userdata
    this.username = 'guest'
    // update userdata if this instance is created after login
    if (userdata) this.setUserdata(userdata)
}
ObjshSDK.User.prototype = {
    setUserdata:function(userdata){
        this.username = userdata.username
    }
}
ObjshSDK.PreferenceItem = function (preferences,name){
    this.name = name
    this.preferences = preferences
    this._value = this.preferences.get(name)
    var self = this
    this.preferences.__defineGetter__(name,function(){return self._value})
    this.preferences.__defineSetter__(name,function(v){
        self._value = v
        self.preferences.set(name,self._value)
    })
}
ObjshSDK.Preferences = function (sdk){
    this.sdk = sdk
    this.values = null
    this._version = 0
    this._sync_timer = 0
    this.sync_from_server()
}
ObjshSDK.Preferences.prototype = {
    sync_from_server:function(){
        //called soon after login
        
        //目前暫時不sync (2019-09-20T06:14:22+00:00)
        this.values = {}
        return

        var self = this //let
        var promise = new $.Deferred() //let
        this.sdk.run_command_line('root.pub.user_preferences.get').done(function(response){
            if (response.retcode != 0) {
                console.warn(response.stderr)
                promise.reject(response.retcode,response.stderr)
                return            
            }
            self.values = response.stdout
            promise.resolve()
        })
        return promise
    },/*
    get_item:function(name){
        var item = new PreferenceItem(this,name)
        return item
    },*/
    get:function(name){
        if (this.name===null) throw "Preferences is not ready to access"
        return this.values[name]
    },
    set:function(key,value){
        this.values[key] = value
        this._version += 1
        this.touch()
    },
    touch:function(){
        //call this to invoke sync to server
        var self = this //let
        if (this._sync_timer==0){
            this._sync_timer = setTimeout(function(){ self.sync_to_server()},3000)
        }
    },
    remove:function(name){
        delete this.values[name]
        this._version += 1
        this.touch()
    },
    clear:function(){
        this.values = {}
        this._version += 1
        this.touch()
    },
    sync_to_server:function(){
        //store to server
        console.warn('sync_to_server is temporary disabled')
        
        /*
        var self = this //let
        var _version = this._version //let
        var command = new Command(self.sdk.metadata.runner_name+'.root.pub.user_preferences.set',[this.values])
        var promise = this.sdk.send_command(command)//let
        promise.done(function(response){
            //console.log(response)
            //console.log('Preferences stored',self._version, _version,self.value)
            if (self._version == _version) {
                self._sync_timer = 0
            }else{
                self._sync_timer = setTimeout(function(){ self.sync_to_server()},3000)
            }
        })
        return promise
        */
    },    
}
