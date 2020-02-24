/*
 w2ui form syntax: https://github.com/vitmalina/w2ui/blob/master/test/form.html
*/
var nodes = {}
var sdk;
var saved_command_lines = null

// manage the "Preview" Panel
function OutputPanelManager(){
    this.source_promise = null
    //this.command = null
}
OutputPanelManager.prototype = {
    set_promise:function(p){
        var self = this
        
        if (p && p === this.source_promise) return //kidding

        if (!p){
            //turn off the current promise's output only
            
            if (this.source_promise) {
                this.empty()
                this.source_promise.switch.on = false
                this.source_promise = null
                //this.command = null
            }
            return
        } else if (this.source_promise) {
            //turn off the current promise's output
            this.source_promise.switch.on = false
            this.source_promise = null
        }
        
        if (p.switch){
            //recover a pre-setup promise
            this.source_promise = p
            this.source_promise.switch.on = true
            return
        }
        // inject to a new promise
        this.source_promise = p
        p.switch = {on:true,command:null}
        p.done(function(response){
            if (!this.switch.on) return
            self.success(response)
        }.bind({switch:p.switch})).progress(function(response){
            if (!this.switch.on) return
            self.progress(response)
        }.bind({switch:p.switch})).fail(function(err){
            if (!this.switch.on) return
            self.failure(err)
        }.bind({switch:p.switch}))
    }
    ,kill_task:function(command){
        if (this.source_promise) this.source_promise.kill()
    }
    ,empty:function(){
        w2ui['layout'].content('preview', '')
        w2ui['layout'].hide('preview')
    }
    ,success:function(response){
        if (w2ui['layout'].get('preview').hidden) w2ui['layout'].show('preview')
        var content = $('<div><pre class="code">' + response + '</pre></div>')
        w2ui['layout'].content('preview', content.html())
    }
    ,failure:function(error){
        if (w2ui['layout'].get('preview').hidden) w2ui['layout'].show('preview')
        var content = '<pre class="code">'+ JSON.stringify(error) + '</pre>'
        w2ui['layout'].content('preview', content)
    }
    ,progress:function(response){
        if (w2ui['layout'].get('preview').hidden) w2ui['layout'].show('preview')
        var content = $('<div><pre class="code">' + response + '</pre></div>')
        w2ui['layout'].content('preview', content.html())    
    }
}
window.output_panel_manager = new OutputPanelManager()

var set_layout = function () {
    var rect = $('#layout')[0].getBoundingClientRect()
    $('#layout')[0].style.height = ($(window).height() - rect.top) + 'px';
    var set_tab_height = function () {
        var tab_content_height = w2ui['layout'].get('main').height - $('#tabs').height() - 25
        if (tab_content_height <= 0) return setTimeout(set_tab_height, 100)
        $('#main-content').height( w2ui['layout'].get('main').height)
        $('#nodes-tab-content').height(tab_content_height)
        $('#events-tab-content').height(tab_content_height)
        $('#logs-tab-content').height(tab_content_height)

        var active_tab = w2ui['layout_main_tabs'].active
        var tab_table_height = tab_content_height
        var tab_table_width = (w2ui['layout'].get('main').width -4 )+ 'px'
        // 27 = $('#node-tabs').height(); node-table has 2 levels tabs
        $('#node-tab-content').height(tab_table_height - 27)
        // adjuest table's height
        w2ui['layout_main_tabs'].click('events-tab')
        $('#events-table').height(tab_table_height)
        $('#events-table').width(tab_table_width)
        w2ui['events-table'].resize();
        $('#bgtasks-table').height(tab_table_height)
        $('#bgtasks-table').width(tab_table_width)
        
        setTimeout(function(){
            //w2ui['layout_main_tabs'].click('nodes-tab')
            w2ui['layout_main_tabs'].click(active_tab)
            
            /* resize logs-tab
            w2ui['layout_main_tabs'].click('logs-tab')
            $('#logs-table').height(tab_table_height)
            $('#logs-table')[0].style.width = tab_table_width
            w2ui['logs-table'].resize();  
            */
        },1000)
    }
    setTimeout(set_tab_height, 100)
}
var show_message = function (body, buttons) {
    if (!buttons) buttons = ['<button class="w2ui-btn" onclick="close_message()">Ok</button>']
    //'main_layout' should always be the toppest layout
    w2ui['main_layout'].message('main', {
        width: 400,
        height: 180,
        body: '<div style="padding: 60px; text-align: center">' + body + '</div>',
        buttons: buttons
    });
}
var close_message = function(){
    w2ui['main_layout'].message('main')
}

function LogsTabManager(){
    this.chunk_size = 20
    this.step_delta = 1 
    this.min_level = 10
    this.log_hours = 1
    // initial to -1, is a workaround
    this.end_ts = -1
}
LogsTabManager.prototype = {
    _retrieve:function(options,callback){
        var self = this
        options.chunk_size = this.chunk_size
        options.min_level = this.min_level
        //w2ui['main_layout'].lock('main', 'loading',true)
        w2utils.lock(document.body,'loading logs',true)
        sdk.log.get(options).progress(function(rows){
            rows.reverse()
            var records = []
            rows.forEach(function(row,i){
                var recid = row.time+'_'+i
                var time = row.time+('['+new Date(row.time*1000).toLocaleTimeString()+']')
                var record = {recid:recid, level:row.level,name:row.name, time:time, text:row.text}//let
                records.push(record)
            })
            w2ui['logs-table'].add(records)
        }).done(function(result){
            w2ui['logs-table'].refresh()
            var start_ts = result.start_ts
            var end_ts = result.end_ts
            self.end_ts = start_ts
            w2utils.unlock(document.body)
            if (callback) callback(result)
        })
    },
    init:function(){
        var self = this
        this._retrieve({last_hours:this.step_delta},function(){
            self.start_to_listen()
        })
    },
    start_to_listen:function(){
        //request last log, than add listener
        sdk.channel.log.add_listener(function (log_obj) {
            var recid = new Date().getTime()/1000//let
            //var time = new Date(log_obj.time*1000).toISOString();
            var time = log_obj.time+('['+new Date(log_obj.time*1000).toLocaleTimeString()+']')
            var record = {recid:recid, level:log_obj.level,name:log_obj.name, time:time, text:log_obj.text}//let
            w2ui['logs-table'].add(record,true)
        })    
    },
    up_a_step:function(){
        var self = this
        var start_ts = this.end_ts - this.step_delta * 3600
        this._retrieve({start_ts:start_ts,end_ts:this.end_ts},function(){
            self.log_hours += 1
            $('#log_hours').html(self.log_hours)
        })
    }
}



function NodeTabsManager(){
    this.tabs = w2ui['node-tabs']
    this.tab_content = $('#node-tab-content')[0]
    this.tab_content.style.overflow = 'auto'
    this.tab_data = {}
    this.counter = 0
    var self = this
    this.tabs.onClick = function(event){self.onClick(event)}
    this.apiinfo
}
NodeTabsManager.prototype = {
    add:function(options,to_head){
        /*
        Create a new tab in the tabs bar.
        @options{
            cmd: <Tree.Branch.FuncName>
            apiinfo: object of an APIInfo
            name: <FuncName>
        }
        @to_head: bool, if true, add to be 1st item.

        /*
        // assign an id
        data.tab_id = new Date().getTime()
        // check existing tab
        for (var tab_id in this.tab_data){
            if (this.tab_data[tab_id].name==node_data.name){
                w2ui['layout_main_tabs'].click('nodes-tab')
                this.tabs.click(tab_id)
                return;
            }
        }*/
        
        this.counter += 1
        var tab_id = 'n'+this.counter 
        this.tab_data[tab_id] = options
        var caption = '<a onclick="window.node_tabs_manager.close(\''+tab_id+'\')" href="#"><span class="fa fa-close"></span></a> ' +  options.name
        if (to_head && this.tabs.tabs.length>0){
            this.tabs.insert(this.tabs.tabs[0].id,{id:tab_id,caption:caption})    
        }else{
            this.tabs.add({id:tab_id,caption:caption})
        }
        w2ui['layout_main_tabs'].click('nodes-tab')
        this.tabs.click(tab_id)
    }
    ,render_tab:function(options){
        
        if (!options) {
            // all tabs are closed
            this.tab_content.innerHTML = ''
            w2ui['main_layout'].hide('right')
            w2ui['layout'].hide('preview')
            return
        }

        // set output panel to display my result
        if (options.promise) {
            window.output_panel_manager.set_promise(options.promise)
            options.promise.done(function(response){
                options.success = response
                options.promise = null
            }).fail(function(err){
                options.failure = err
                options.promise = null
            })
            //.progress(function(_,command){
            //    options.command = command
            //})
        }else if (options.success){
            window.output_panel_manager.set_promise(null)
            window.output_panel_manager.success(options.success)
        }else if (options.failure){
            window.output_panel_manager.set_promise(null)
            window.output_panel_manager.failure(options.failure)
        }else{
            window.output_panel_manager.set_promise(null)
        }

        //generate page for APIInfo
        var html_apiinfo = ['<div class="apiinfo">']
        html_apiinfo.push('<div class="title-bar">')
        //html_apiinfo.push(options.cmd)
        html_apiinfo.push('<a href="" id="">Docstring</a>')
        html_apiinfo.push('<a href="" id="">Source</a>')
        html_apiinfo.push('<a href="" id="">Examples</a>')
        html_apiinfo.push('<a href="" id="refresh-apiinfo-btn">Refresh</a>')
        html_apiinfo.push('</div>')
        html_apiinfo.push('<div class="comment">'+marked(options.apiinfo.Comment)+'</div>')
        html_apiinfo.push('</div>')

        w2ui['main_layout'].content('right','<div id="api-call-form"></div>')
        // show node's class doc
        this.tab_content.innerHTML = html_apiinfo.join('')

        _.defer(function(){
            // generate a form to call this api
            window.api_comment2w2form(options.apiinfo.Comment,'#api-call-form',options)
            w2ui['main_layout'].show('right')
            $('#refresh-apiinfo-btn').on('click',function(evt){
                evt.preventDefault()
                sdk.tree.call("$.RescanAPIInfo",[options.cmd]).done(function(changed){
                    for(var apiName in changed){
                        var nodeid = sdk.tree.treeName+'.'+apiName
                        var item = w2ui['sidebar'].get(nodeid)
                        item.apiinfo = changed[apiName]
                        if (item.id == options.cmd){
                            //update current call's comment
                            options.ArgsKw = {Args:[],kw:{}} //reset existing args and kw's value
                            $('.apiinfo .comment').html(marked(item.apiinfo.Comment))
                            window.api_comment2w2form(item.apiinfo.Comment,'#api-call-form',options)
                        }
                    }
                }).fail(function(err){
                    alert(JSON.stringify(err))
                })
            })    
        })

    }
    ,obsoldete_add:function(node_id, node_data){
        /*
         * node_id is actually the hierarchical absolute path of this node
         */        
        
         // check existing tab
        for (var tab_id in this.tab_data){
            if (this.tab_data[tab_id].name==node_data.name){
                w2ui['layout_main_tabs'].click('nodes-tab')
                this.tabs.click(tab_id)
                return;
            }
        }
        // create new tab
        this.counter += 1
        var tab_id = 'n'+this.counter 
        this.tab_data[tab_id] = [node_id, node_data]
        var caption = '<a onclick="window.node_tabs_manager.close(\''+tab_id+'\')" href="#"><span class="fa fa-close"></span></a> ' +  node_data.name
        if (this.tabs.tabs.length==0) this.tabs.add({id:tab_id,caption:caption})
        else this.tabs.insert(this.tabs.tabs[0].id,{id:tab_id,caption:caption})
        w2ui['layout_main_tabs'].click('nodes-tab')
        this.tabs.click(tab_id)
    },
    close:function(tab_id){
        this.tabs.remove(tab_id)
        var options = this.tab_data[tab_id]

        if (options.promise){ //the call in this tab has been executed
            //turn off output
            options.promise.switch.on = false
            // cancel task if it is running but not yte completed.
            // but keep it running if it is a background task
            if (options.promise && !(options.success || options.failure) && (!options.promise.background)){
                options.promise.kill()
            }
        }
        delete this.tab_data[tab_id]
        
        if (tab_id !=  this.active_tab_id) return

        // choose a new tab
        var tab_counter = parseInt(tab_id.substr(1))
        var max_smaller = -1
        var min_larger = 99999
        for (var tab_id in this.tab_data){
            var c =  parseInt(tab_id.substr(1))
            if (c < tab_counter && c > max_smaller) max_smaller = c
            else if (c > tab_counter && c < min_larger) min_larger = c
        }
        if (max_smaller > -1 ) this.tabs.click('n'+max_smaller)
        else if (min_larger < 99999) this.tabs.click('n'+min_larger)
        else {
            this.active_tab_id = null
            this.render_tab(null)
        }
    },
    onClick: function(event){
        this.active_tab_id = event.target
        this.render_tab(this.tab_data[event.target])
    },
    count:function(){
        var c = 0
        for (var _ in this.tab_data){c += 1}
        return c
    },    
    old_restore_argspec:function(argspec){
        args = argspec[0].slice()
        //remove leading "self"
        if (args.length && args[0]=='self') args.splice(0,1)
        //remove leading "task" (auto-given bi "require_task" flag)
        if (args.length && args[0]=='task') args.splice(0,1)
        if (argspec[3] && argspec[3].length){
            var start_idx = args.length - argspec[3].length
            argspec[3].forEach(function(v,idx){
                args[start_idx+idx] += ('=' + ((typeof(v)==typeof(1) || typeof(v)==typeof(1.0)) ? v : (v===null ? 'None' : (typeof(v)=='boolean' ? (''+v).replace(/\b\w/g, l => l.toUpperCase()) : '"'+v+'"'))))
            })
        }
        if (argspec[1]) args.push('*'+argspec[1])
        if (argspec[2]) args.push('**'+argspec[2])
        return (args.length ? '('+args.join(', ')+')' : '')
    },    
    render:function(node_meta){
        //python版的render是render一個node(aka branch,有好個exports)
        var node_id = node_meta[0]
        var node_data = node_meta[1]
        // change input line
        var self = this
        var node_name = node_data.name
        var command_line = ObjshSDK.metadata.runner_name + '.nodes.' + node_name

        var counter = 0
        var node_records = []
        var id_to_expand = []

        var top_record = {recid:counter,kind:'Node',type:'node',name:node_name,doc:node_data.doc,w2ui:{children:[]}}
        counter += 1
        node_records.push(top_record)
        top_record.w2ui.children.push({recid:counter,type:'class', kind:'Class',name:node_data.classname,doc:node_data.file})
        counter += 1

        id_to_expand.push(counter)
        var export_records = {recid:counter,kind:'Methods',name:'',doc:'',w2ui:{children:[]}}
        counter += 1
        node_records.push(export_records)
        var keys = Object.keys(node_data.exports)
        keys.sort()
        keys.forEach(function (method_name,idx) {
            var argspec = node_data.exports[method_name].argspec
            //console.log(method_name,argspec,'<<')
            var method_name_args
            if (argspec){
                //restore the arguments list of this method
                method_name_args = method_name+ self.restore_argspec(argspec)
            }
            else method_name_args = method_name

            export_records.w2ui.children.push({recid:counter,type:'method',kind:'',name:method_name_args,doc:node_data.exports[method_name].doc,argspec:node_data.exports[method_name].argspec})
            counter += 1
        })

        var keys = Object.keys(node_data.resources)
        if (keys.length){
            id_to_expand.push(counter)
            var resource_records = {recid:counter,kind:'Resources',name:'',doc:'',w2ui:{children:[]}}
            counter += 1
            node_records.push(resource_records)
            keys.sort()
            keys.forEach(function (resource_name,idx) {
                //console.log(method_name,argspec,'<<')
                resource_records.w2ui.children.push({recid:counter,type:'resource',kind:'',name:resource_name+'(request)',doc:node_data.resources[resource_name].doc,resource_name:resource_name})
                counter += 1
            })    
        }

        id_to_expand.push(counter)
        var event_records = {recid:counter,kind:'Events',name:'',doc:'',w2ui:{children:[]}}
        counter += 1
        node_records.push(event_records)
        var keys = Object.keys(node_data.events)
        keys.sort()
        keys.forEach(function (key,idx) {
            var value = node_data.events[key]
            event_records.w2ui.children.push({recid:counter,type:'event',kind:'',name:key,doc:value})
            counter += 1
        })

        id_to_expand.push(counter)
        var exception_records = {recid:counter,kind:'Exceptions',name:'',doc:'',w2ui:{children:[]}}
        counter += 1
        node_records.push(exception_records)
        var keys = Object.keys(node_data.exceptions)
        keys.sort()
        keys.forEach(function (key,idx) {
            var value = node_data.exceptions[key]
            if (typeof(value)=='object'){
                value = '('+value.retcode+', '+ value.message +')'
            } 
            exception_records.w2ui.children.push({recid:counter,type:'exception', kind:'',name:key,doc:value})
            counter += 1
        })

        $('#exports').attr('node_name', node_name)
        $('#exports').attr('command_line', command_line)
        
        // show node's class doc
        var content = '<div id="node-info"><div id="node-records" style="overflow:auto;width:100%;height:'+(30+counter*24)+'px"></div></div>'
        this.tab_content.innerHTML = content

        if (w2ui['node-records']) w2ui['node-records'].destroy()
        $('#node-records').w2grid({
            name:'node-records',
            show: {
                toolbarAdd: true,
                toolbarDelete: true,
                toolbar:false, //disable temporary
                lineNumbers: true,
                footer: false,
                toolbar:true,
                toolbarSearch:false,
                toolbarReload:false,
                toolbarColumns:false,
                toolbarInput:false,
            },
            columns: [                
                { field: 'kind', caption: 'Kind',size:'10%'},
                { field: 'name', caption: 'Name',size:'40%'},
                { field: 'doc', caption: 'Description',size:'50%'},
            ],
            onAdd: function (event) {
                w2alert('add');
            },
            onDelete: function (event) {
                console.log('delete has default behavior');
            },                
            records:node_records,
            onClick:function(evt){
                var record = null
                var recid = parseInt(evt.recid)
                for (var i=0;i<node_records.length;i++){
                    //lookup the last item or if its recid is greater than trageted recid
                    if (node_records[i].recid==recid){
                        record = node_records[i]
                    }
                    else if (i == node_records.length-1 || node_records[i].recid > recid ){
                        var n = ( node_records[i].recid > recid) ? (i-1) : (node_records.length-1)
                        node_records[n].w2ui.children.some(function(rec){
                            if (rec.recid==recid){
                                record = rec
                                return true
                            }
                        })
                        break
                    }
                }
                if (record.type=='method'){
                    if (evt.column==1){
                        $('#command-line').val(sdk.metadata.runner_name+'.nodes.'+node_name+'.'+record.name)
                    }
                    else if (evt.column==2){
                        //description
                        var link;
                        if (record.argspec){
                            window._gen_docstring_sample = function(){
                                var rows = window.gen_doc_sample.gen(record.argspec,record.doc)
                                var content = '<pre id="sampledoc" style="margin:0;border:solid 1px #e0e0e0;padding:10px;">'+rows+'</pre><div style="padding-bottom:5px" id="copy2clipboard"></div>'
                                $('#docstring_template').html(content)
                                gen_doc_sample.make_copy_button(document.getElementById("copy2clipboard"), document.getElementById("sampledoc"));
                            }
                            link = '<div id="docstring_template"><a href="#" style="font-size:14px" onclick="window._gen_docstring_sample()">Generate docstring for this method (experimental)</a></div>'
                        }
                        else link = ''
                        self.show_content('<pre>'+(record.doc||'(empty)')+'</pre>'+link)
                    }
                }
                else if (record.type=='class' && evt.column==2){
                    //open source
                    self.show_source(node_data.path)
                }
                else if (record.type=='node'||record.type=='event'||record.type=='exception'){
                    self.show_content('<pre>'+(record.doc||'(empty)')+'</pre>')
                }
                else if (record.type='resource'){
                    // exclude "root." 
                    var middle_path = node_id.split('.')
                    middle_path.shift()
                    // exclude NodeValues's owner_node's name (so node_name) is not included in the following line
                    var url = '/'+[sdk.metadata.resource_route_name,middle_path.join('/'),record.resource_name].join('/')
                    open(url)
                }
            }
        })
        w2ui['node-records'].resize()
        id_to_expand.forEach(function(recid){
            w2ui['node-records'].expand(recid)
        })
    },
    show_source:function(node_path) {
        window.utility.loading(true)
        var self = this
        var command = Command.from_line(sdk.metadata.runner_name + '.root.playground.gui_get_source ' + node_path)
        sdk.send_command(command).done(function (response) {
            window.utility.loading(false)
            var source = response.retcode == 0 ? response.stdout : response.stderr
            var content = '<pre>' + source + '</pre>'
            self.show_content(content)
        })
    },
    show_content:function(content){
        w2ui['main_layout'].content('right','<div style="margin:0px;min-height:100%;padding:10px;background-color:transparent;">' + content + '</div>' )
        w2ui['main_layout'].show('right')
    }
}
$(window).resize(function () {
    set_layout()
})

/* initialization */
$(function () {
    
    //markdown
    marked.setOptions({
        headerIds:false,
        breaks:true,
    })  

    window.utility = new Utility()

    $('#loginform').w2form({
        name: 'loginform',
        header: 'API Development Playground',
        height: '200px',
        fields: [
            { name: 'username', type: 'text',  },
            { name: 'password', type: 'password'},
        ]
        ,record:{
            username:'admin'
            ,password:''
        }
        ,actions: {
            login: function () {
                var username = $('#username').val()
                var password = $('#password').val()
                window.login(username, password)
            }
        }
    })

    // initial layout


    var pstyle = 'background-color: #ffffff;overflow:hidden';
    var main_content = '<div style="width:100%" id="main-content"></div>'
    $('#layout').w2layout({
        name: 'layout',
        panels: [
            { type: 'left', size: 200, resizable: true, style: pstyle, content:''},
            { type: 'main', size: '75%', style: 'background-color: #ffffff;', content: main_content },
            { type: 'preview', size: '25%', style: 'background-color: #efefef;', content: '',
                    /*
                    toolbar: {
                        name: 'fg_toolbar',
                        id: 'fg_toolbar',
                        items: [
                            {
                                type: 'button', id: 'cancelcmd', icon: 'fa fa-stop', text: 'Cancel', disabled: true,
                                tooltip: function (item) {
                                    return 'Cancel the running command';
                                },
                                onClick: function (event) {
                                    window.output_panel_manager.cancel_task()
                                }
                            },
                            {
                                type: 'button', id: 'tobgcmd', icon: 'fa fa-forward', text: 'To Background', disabled: true,
                                tooltip: function (item) {
                                    return 'Send to background';
                                },
                                onClick: function (event) {
                                    if (!window.running_fg_command) return
                                    var task_id = window.running_fg_command.id
                                    window.running_fg_command = null
                                    w2ui.toolbar.enable('runcmd') //main toolbar
                                    var fg_toolbar = w2ui['layout'].get('preview').toolbar
                                    fg_toolbar.disable('tobgcmd')
                                    fg_toolbar.disable('cancelcmd')
                                    window.send_to_background(task_id)
                                }
                            },
                            { type: 'spacer' },
                            {
                                type: 'button', id: 'close_panel', icon: 'fa fa-close', text: '', disabled: false,
                                tooltip: function (item) {
                                    return 'Close';
                                },
                                onClick: function (event) {
                                    w2ui['layout'].hide('preview')
                                }
                            },                            
                        ]
                    }*/
            },
            { type: 'right', size: '25%', hidden: true, resizable: true, style: pstyle, content: '', title: 'Tasks<span id="refreshing_mark"></span> <a onclick="w2ui[\'layout\'].hide(\'right\')" style="float:right;right:20px"><span class="fa fa-angle-double-right"></span> Hide</a>' },
            { type: 'bottom', size: 15, style: pstyle, resizable: false, content: '<div class="page-bottom">build:<span class="objsh_version"></span></div>' }
        ]
    });
    w2ui['layout'].hide('preview')

    var tabs_content = '<div id="tabs"></div>'+
        '<div class="tab-content" id="nodes-tab-content"><div id="node-tabs"></div><div id="node-tab-content"></div></div>'+
        '<div class="tab-content" id="bgtasks-tab-content"><div id="bgtasks-table"></div></div>'+
        '<div class="tab-content" id="events-tab-content"><div id="events-table"></div></div>'+
        '<div class="tab-content" id="logs-tab-content"><div id="logs-table"></div></div>'
    
        $('#main-content').w2layout({
        name: 'main_layout',
        panels: [
            { type: 'main', size: '75%', style: pstyle , content:tabs_content},
            { type: 'right', size: '25%', style:'padding:2px;border:solid 1px #e0e0e0;background-color:white;',title: '<a onclick="w2ui[\'main_layout\'].hide(\'right\')" style="float:right;right:20px"><span class="fa fa-angle-double-right"></span> Hide</a>' },
        ]        
    })
    
    w2ui['main_layout'].hide('right')

    //var active_node_tab = null
    $('#node-tabs').w2tabs({
        name:'node-tabs',
        tabs:[]
    })
    window.node_tabs_manager = new NodeTabsManager()

    var active_tab = 'nodes-tab'
    $('#tabs').w2tabs({
        name: 'layout_main_tabs',
        active: 'nodes-tab',
        tabs: [
            { id: 'nodes-tab', caption: 'Calls' },
            { id: 'bgtasks-tab', caption: 'Background Tasks' },
            { id: 'events-tab', caption: 'Messages' },
            { id: 'logs-tab', caption: 'Logs' },
        ],
        onClick: function (event) {
            $('#' + active_tab + '-content')[0].style.display = 'none'
            $('#' + event.target + '-content')[0].style.display = 'block'
            active_tab = event.target
            var hideCallsTabPanel = function(){
                w2ui['main_layout'].hide('right')
                w2ui['layout'].hide('preview')
            }
            if (event.target=='nodes-tab') {
                if (window.node_tabs_manager.count() > 0)  w2ui['main_layout'].show('right')

            }else if (event.target=='bgtasks-tab') {
                hideCallsTabPanel()
                sdk.tree.call('$.ListUserTasks').done(function(response){
                    var records = []
                    response.forEach(function(cmdInfo,i){
                        //id, cmdpath, username, args, kw, ctime
                        var values = cmdInfo.split('\t')
                        values[3] = values[3].replace('&',', ')
                        records.push({
                            recid:i,
                            hooked: '',
                            time:values[5],
                            cmdID: values[0],
                            cmdPath: values[1],
                            argsKw:(values[3] || '')+( (values[3] && values[4] )? ', ' : '')+(values[4] || ''),
                        })
                    })
                    if (w2ui['bgtasks-table']) w2ui['bgtasks-table'].destroy()
                    $('#bgtasks-table').w2grid({
                        name: 'bgtasks-table', 
                        show: {
                            lineNumbers: true,
                            footer: false,
                            toolbar:true,
                            lineNumbers: true,
                            footer: false,
                            toolbar:true,
                            toolbarSearch:false,
                            toolbarReload:false,
                            toolbarColumns:false,
                            toolbarInput:false,
                        },
                        columns: [
                            { field: 'hooked', caption:'Hooked', size:'10px'},
                            { field: 'time', caption: 'Timestamp', size:'10%',info: true},
                            { field: 'username', caption: 'User',size:'10%'},
                            { field: 'cmdID', caption: 'ID',size:'10%'},
                            { field: 'cmdPath', caption: 'Call',size:'30%'},
                            { field: 'argsKw', caption: 'Args & Kw',size:'40%'},
                        ],
                        records: records,
                        toolbar:{
                            name:'bgtasks-table-toolbar',
                            id:'bgtasks-table-toolbar',
                            items:[
                                { type: 'button', id: 'kill-btn', icon: 'fa fa-stop-circle', text: 'Kill', disabled:false,
                                    tooltip: function (item) {
                                        return 'Kill this task';
                                    },
                                    onClick: function (event) {
                                        var recid = w2ui['bgtasks-table'].getSelection()
                                        
                                        var records =  w2ui['bgtasks-table'].get(recid)
                                        if (records.length){
                                            sdk.tree.kill({
                                                name:records[0].cmdPath,
                                                id:records[0].cmdID
                                            }).done(function(){
                                                w2ui['bgtasks-table'].delete(true)
                                                w2alert("Job Killed")
                                            }).fail(function(err){
                                                w2alert(JSON.stringify(err))
                                            })
                                        }
                                    }
                                }, 
                                { type: 'button', id: 'hook-btn', icon: 'fa fa-play-circle', text: 'Hook', disabled:false,
                                    tooltip: function (item) {
                                        return 'Hook up to this task';
                                    },
                                    onClick: function (event) {
                                        var recid = w2ui['bgtasks-table'].getSelection()                                        
                                        var records =  w2ui['bgtasks-table'].get(recid)
                                        if (records.length){
                                            var cmdID = records[0].cmdID
                                            records[0].hooked = 'Y'
                                            if (!window.HookTasks) window.HookTasks = {}
                                            window.HookTasks[cmdID] = sdk.tree.hook(cmdID).progress(function(stdout){
                                                window.add_event({payload:stdout, source:cmdID})                                          
                                            }).fail(function(err){
                                                records[0].hooked = ''
                                                window.add_event({payload:JSON.stringify(err), source:cmdID})
                                            })
                                            w2ui['bgtasks-table'].refreshRow(records[0].recid)
                                        }
                                    }
                                },   
                                { type: 'button', id: 'unhook-btn', icon: 'fa fa-pause-circle', text: 'Unhook', disabled:false,
                                    tooltip: function (item) {
                                        return 'UN hook up to this task';
                                    },
                                    onClick: function (event) {
                                        var recid = w2ui['bgtasks-table'].getSelection()                                        
                                        var records =  w2ui['bgtasks-table'].get(recid)
                                        
                                        if (records.length){
                                            var cmdID = records[0].cmdID
                                            var task = window.HookTasks[cmdID]
                                            if (task){
                                                task.kill()
                                                records[0].hooked = ''
                                                w2ui['bgtasks-table'].refreshRow(records[0].recid)
                                                delete window.HookTasks[cmdID]
                                                window.add_event({payload:'Unhook '+cmdID,source:'Playground'})
                                            }
                                        }
                                    }
                                },                                                                             
                            ]
                        }
                    })
                    _.defer(function(){
                        w2ui['bgtasks-table'].refresh()
                    })                 
                    
                })
            } else if (event.target=='events-tab') {
                hideCallsTabPanel()
                w2ui['events-table'].refresh()
            } else if (event.target=='logs-tab') {
                hideCallsTabPanel()
                // this is a quick-and-dirty work-around to call window.logs_tab_manager.init()
                // at the 2nd time been clicked. (1st-time is called by script)
                if (window.logs_tab_manager.end_ts == -1) {
                    window.logs_tab_manager.end_ts = 0
                }
                else if (window.logs_tab_manager.end_ts == 0){
                    //do initialization retrieve
                    window.logs_tab_manager.init()
                }
                else w2ui['logs-table'].refresh()
            } 
        }
    })
    $('#events-table').w2grid({
        name: 'events-table', 
        show: {
            lineNumbers: true,
            footer: false,
            toolbar:true,
            toolbarSearch:false,
            toolbarReload:false,
            toolbarColumns:false,
            toolbarInput:false,
        },
        columns: [                
            { field: 'time', caption: 'Timestamp', size:'10%',info: true},
            { field: 'name', caption: 'Event Name',size:'10%'},
            { field: 'source', caption: 'Source Node',size:'20%'},
            { field: 'payload', caption: 'Payload',size:'60%'}
        ],
        records: [],
        toolbar:{
            name:'events-table-toolbar',
            id:'events-table-toolbar',
            items:[
                { type: 'button', id: 'clear-log-cmd', icon: 'fa fa-ban', text: 'Clear', disabled:false,
                    tooltip: function (item) {
                        return 'Clear this log table';
                    },
                    onClick: function (event) {
                        w2ui['events-table'].clear()
                    }
                },                
            ]
        }
    })

    var add_line_timer = 0
    window.add_event = _.throttle(function(options){
            //truncate too long message
            if (options.payload.length > 100){
                var payload = options.payload.substr(0,100)+ '...'
            }else{
                var payload = options.payload
            }
            var now = new Date()
            var recid = now.getTime()
            var time = (now.getMonth()+1)+'-'+now.getDate()+' '+now.getHours()+':'+now.getMinutes()+':'+now.getSeconds()
            var record = {recid:recid, payload:payload, time:time, name:options.name || '', source:options.source || ''}
            if (w2ui['events-table'].records.length > 50){
                w2ui['events-table'].clear()//true: no refresh
                w2ui['events-table'].add({recid:recid+1,payload:'-----ove-50-lines-auto-clear-----'},true)
            }
            w2ui['events-table'].add(record,true)
            //if (add_line_timer) clearTimeout(add_line_timer)
            //add_line_timer = setTimeout(function(){},100)
        }
    ) 
    //_.defer(function(){ w2ui['events-table'].refresh()})

    /*
    window.logs_tab_manager = new LogsTabManager()    
    $('#logs-table').w2grid({
        name: 'logs-table', 
        toolbar: {
            name:'logs-table-toolbar',
            id:'logs-table-toolbar',
            items:[
                { type: 'button', id: 'downloadcmd', icon: 'fa fa-chevron-circle-down', text: 'Download', disabled:false,
                    tooltip: function (item) {
                        return 'Download log file';
                    },
                    onClick: function (event) {
                        w2alert('Not implemented')
                        / *
                        url = sdk.get_command_url('root.pub.log.download')
                        open(url)
                        * /
                    }
                },
                {type:'break'},
                { type: 'menu-radio', id: 'log_level', icon: 'fa fa-star',
                    text: function (item) {
                        var text = item.selected;
                        return  text
                    },
                    selected: 'info',
                    items: [
                        { id: 'info', text: 'info', icon: '' },
                        { id: 'debug', text: 'debug', icon: '' },
                        { id: 'warn', text: 'warn', icon: '' },
                        { id: 'error', text: 'error', icon: '' },
                        { id: 'critical', text: 'critical', icon: ''}
                    ]
                },                
                { type: 'html',  id: 'write_log_line',
                    html: function (item) {
                        var html =
                        '<div style="padding: 3px 10px;">'+
                        '    <input id="log-line"'+
                        '     placeholder="message to log"'+
                        '     style="width:250px;padding: 3px; border-radius: 2px; border: 1px solid silver" value=""/>'+
                        '</div>';
                        return html;
                    }
                },
                { type: 'button', id: 'write-log-cmd', icon: '', text: 'Log', disabled:false,
                    tooltip: function (item) {
                        return 'Write a line of log to server';
                    },
                    onClick: function (event) {
                        var level = this.get('log_level').selected
                        var line = $('#log-line').val()
                        sdk.log._msg(level,line)
                    }
                },
                {type:'break'},
                { type: 'button', id: 'last-log-cmd', icon: 'fa fa-arrow-up', text: '1Hr(<span id="log_hours">1</span>)', disabled:false,
                    tooltip: function (item) {
                        return 'Click to show more logs, 1 hour ago/click'
                    },
                    onClick: function (event) {
                        this.disabled = true
                        window.logs_tab_manager.up_a_step()
                    }
                },
                {type:'break'},
                { type: 'button', id: 'clear-log-cmd', icon: 'fa fa-ban', text: 'Clear', disabled:false,
                    tooltip: function (item) {
                        return 'Clear this log table';
                    },
                    onClick: function (event) {
                        w2ui['logs-table'].clear()
                    }
                },
            ]},        
        show: {
            lineNumbers: true,
            footer: false,
            toolbar:true
        },
        columns: [                
            //{ field: 'recid', caption: 'No', size: '60px', sortable: true },
            { field: 'time', caption: 'Timestamp', size: '230px', info: true},
            { field: 'level', caption: 'Level', size: '40px'},
            { field: 'name', caption: 'Name', size: '80px'},
            { field: 'text', caption: 'Content',size:'80%'}
        ],
        records: [ ]
    })
    //enable log
    $('#log-line').click(function(evt){
        var level = w2ui['logs-table'].get('log-level')
        //onchange="sdk._msg($(\'#log-line\').val());this.value=\'\'"
        var line = $('#log-line')
        console.log([level,line])
    })
    */

    // decorate the bottom
    var bottom_parent = $(w2ui['layout'].box).find('.page-bottom')[0].parentNode
    bottom_parent.style.padding = '0px'
    bottom_parent.style.backgroundColor = '#efefef'

    $().w2layout({
        name: 'right_inner_layout',
        panels: [
            { type: 'main', size: '25%', style: pstyle },
            {
                type: 'preview', size: '75%', resizable: true, style: pstyle, content: '',
                toolbar: {
                    name: 'bg_toolbar',
                    items: [
                        {
                            type: 'button', id: 'bg_cancelcmd', icon: 'fa fa-stop', text: 'Cancel', disabled: true,
                            tooltip: function (item) {
                                return 'Cancel the background command';
                            },
                            onClick: function (event) {
                                if (!window.last_watching_task_id) {
                                    console.warn('no watching task')
                                    return
                                }                                
                                window.cancel_task().done(function () {
                                    setTimeout(function () { window.list_tasks() }, 1000)
                                })
                            }
                        },
                        {
                            type: 'button', id: 'bg_bgcmd', icon: 'fa fa-forward', text: 'To Background', disabled: true,
                            tooltip: function (item) {
                                return 'Cancel the background command';
                            },
                            onClick: function (event) {
                                if (!window.last_watching_task_id) {
                                    console.warn('no watching task')
                                    return
                                }

                                var fg_toolbar = w2ui['right_inner_layout'].get('preview').toolbar
                                fg_toolbar.disable('bg_bgcmd')

                                var task_id = window.last_watching_task_id
                                window.send_to_background(task_id)
                            }
                        }
                    ]
                }
            },
        ]
    });
    w2ui['layout'].content('right', w2ui['right_inner_layout'])    

    //w2alert('Changed records are displayed in the console');
    build_toolbar()

    /* If you are going to connect another host, use the following line */
    var port = window.location.port 
    if (!port) port = window.location.protocol=='https:' ? 443 : 80
    var hostname = window.location.hostname //let

    //figure out accesspath by url
    var paths = window.location.pathname.split('/') //let
    var p = paths.indexOf('websdk')
    var access_path = '/'+(p>0 ? paths[p-1] : '')

    window.sdk = GetSDKSingleton()
    window.treedata = [ //tree 是預先知道的
        {name:'Playground',url:'/playground/tree',selected:true},
        {name:'Sidebyside',url:'/sbs'},
        {name:'UnitTest',url:'/objsh/tree'},
        {name:'NHIRD',url:'/nhirdws'},
    ]
    var has_been_login = false
    window.login = function (username, password) {
        var _username = username
        if (_username) $('#login-message').html('')
        sdk.login({
                url:  '/playground/login',
                username: username,
                password: password,
                method:'POST'
            })
            .fail(function(){
                if (username) w2alert("Login Failure")
            })
            .done(function (user) {
                // login success
                $('#login-username').html(user.username)
                $('#loginform').hide()
                $('#connected_content')[0].style.visibility = 'visible'
               
                var treedata =  window.treedata[0]
                sdk.useTree(treedata.name,treedata.url)
                sdk.on('tree:open', function(){
                    sdk.tree.call("$.Layout").done(function(tree_layout){
                        build_sidebar(tree_layout)
                        set_layout() //設定各種尺寸，不然會看不到ELEMENT

                        /*
                        //更新protobuf 參數的選單
                        //因為toolbar取消了，這個暫時也取消
                        var pbtype_items = []
                        for (var typeName in sdk.tree.protobuf.pbTypes){
                            pbtype_items.push({id:typeName, text:typeName})
                        }
                        w2ui['toolbar'].get('pb-message').items = pbtype_items
                        w2ui['toolbar'].refresh('pb-message')
                        */
                    }).fail(function(err){
                        console.log(err)
                    })
                    // Hook announce to event table
                    sdk.tree.onannouce = function(stdout){
                        window.add_event({payload:stdout, name:'Announce'})
                    }
                })
                sdk.on('tree:close',function(){
                    console.log('tree close')
                })
                /*
                has_been_login = true
                sdk.connect().progress(function (state) {
                    //console.log(state)
                    switch (state) {
                        case 'onopen':
                            $('.server_name').html(ObjshSDK.metadata.server_name)
                            $('.objsh_version').html(ObjshSDK.metadata.objsh_version)
                            w2ui['layout'].unlock('left')
                            $('#connected_content')[0].style.opacity = 1
                            build_layout()
                            set_layout()
                            / * for testing auto_reconnect only * /
                            // setTimeout(function(){sdk.ws.close();console.log('auto closed')},5000)                        
                            break
                        case 'onclose':
                            if (sdk.is_authenticated) {
                                // disconnected from server, maybe
                                // 1. server in restarting process
                                // 2. session has logout by other webpage
                                $('#connected_content')[0].style.opacity = 0.1
                                var mesg = (sdk.auto_relogin_connect || sdk.auto_reconnect) ? 'reconnecting' : 'disconnected'
                                w2ui['layout'].lock('left', mesg)
                                //setTimeout(function () { window.login() }, 3000)
                            }
                            else {
                                console.log('user logout')
                            }
                            break
                    }
                })
                */
            },
            function (code, reason) {
                // login failure , this is called 
                console.log('rejected, code=', code, ', reason', reason, ', sdk.is_authenticated', sdk.is_authenticated)
                if (_username) {
                    $('#login-message').html(reason)
                    setTimeout(function(){
                        $('#login-message').html('')
                    },2000)
                }
                if (code == 0) {
                    setTimeout(function () { window.login() }, 3000)
                }
                else if (code == 403) {
                    // authentication error or logout
                    //console.log('unauthenticated',has_been_login)
                    if (has_been_login) window.location.reload()
                }
            }
        )
    }
    window.login()

});
function update_saved_command_lines() {
    var items = []
    saved_command_lines.forEach(function (line, idx) {
        if (!line.trim()) return
        items.push({ id: idx, text: line, icon: '' })
    })
    w2ui['toolbar'].set('savedcmd', { items: items })
}

function set_command_line(line) {
    $('#command-line').val(line)
}
function show_on_right(ele, content) {
    $(ele).w2overlay('<div style="padding:10px">' + content + '</div>');
}
function add_export_to_command_line(text) {
    $('#command-line').val($('#exports').attr('command_line') + text)
}
/*
 * Common utilities start
 */
function Utility() {
    this.loadingCount = 0
    this.init()
}
Utility.prototype = {
    init: function () {
    },
    localI18n: function (s) { return s },//dummy
    loading: function (yes) {
        //@param yes: boolean or string
        var title;
        if (yes) title = (typeof (yes) == 'string') ? yes : this.localI18n('Loading')
        if ((this.loadingCount < 0 && !yes) || (this.loadingCount > 0 && yes)) {
            this.loadingCount += yes ? 1 : -1;
            return;
        }
        this.loadingCount += yes ? 1 : -1;
        if (this.loadingCount == 1) {
            w2popup.open({
                width: 220,
                height: 60,
                modal: true,
                body: '<div class="w2ui-centered"><div style="padding: 10px;">' +
                    '        <div class="w2ui-spinner" ' +
                    '            style="width: 22px; height: 22px; position: relative; top: 6px;"></div>' +
                    '        Loading... ' +
                    '</div></div>'
            })
        }
        else {
            w2popup.close();
        }
    },
}
/*
 * DocSampleGenerator
 */
function DocSampleGenerator() {
}
DocSampleGenerator.prototype = {
    parse_docstring:function(s){
        var topics = {
            description:[],
            args:[],
            returns:[],
            yields:[],
            raises:[],
            examples:[],
            notes:[]
        }
        if (!s) return topics
        var topic = ''
        var in_top_description = true
        s.split('\n').forEach(function(line){
            var _line = line.trim().toLowerCase()
            if (_line==''){
                if (in_top_description) topics['description'].push('')
                return
            }
            for (var key in topics){
                if (_line.indexOf(key+':')==0){
                    topic = key
                    in_top_description = false
                    return
                }
            }
            if (in_top_description)topics['description'].push(line)
            else if (topic) topics[topic].push(line)
        })
        return topics
    },
    gen: function (argspec,docstring) {
        /* generate a python doc sample by argspec
         * @param argspec = [args, varargs, keywords, defaults]
         * @param dosctriong: existing docstring
         */
        if (!argspec) return docstriong || ''
        var topics = this.parse_docstring(docstring)
        var padding = '    '
        var rows = ['"""']
        if (topics.description.length){
            topics.description.forEach(function(line){rows.push(line)})
        }
        else rows.push('(descriptions)')
        rows.push('')

        // check if there is methods exported
        var args = []
        var default_start_idx = argspec[3] ? (argspec[0].length - argspec[3].length) : -1
        argspec[0].forEach(function (arg, idx) {
            if (arg == 'self') return
            var default_value = default_start_idx >= 0 && idx >= default_start_idx ? (argspec[3][idx - default_start_idx]) : undefined
            if (typeof (default_value) != 'undefined') {
                default_value_str = default_value == null ? 'None' : new String(default_value)
                args.push({ name: arg, default_value: default_value_str })
            }
            else {
                args.push({ name: arg })
            }
        })
        if (argspec[1]) args.push({ name: argspec[1], args_star: true })
        if (argspec[2]) args.push({ name: argspec[2], kw_star: true })
        if (args.length) {
            rows.push('Args:')
            //append existing args (not good idea)
            if (topics.args.length){
                topics.args.forEach(function(line){rows.push(line)})
            }
            this._gen_args(args, rows, padding)
        }
        
        rows.push('Yields:')
        if (topics.yields.length){topics.yields.forEach(function(line){rows.push(line)})}        
        rows.push(padding)
        rows.push('Notes:')
        if (topics.notes.length){topics.notes.forEach(function(line){rows.push(line)})}   
        rows.push(padding)
        rows.push('Examples:')
        if (topics.examples.length){topics.examples.forEach(function(line){rows.push(line)})}   
        rows.push(padding)
        rows.push('Returns:')
        if (topics.returns.length){topics.returns.forEach(function(line){rows.push(line)})}   
        rows.push(padding)
        rows.push('Raises:')
        if (topics.raises.length){topics.raises.forEach(function(line){rows.push(line)})}   
        else rows.push(padding + 'Error:(descriptions)')
        rows.push('"""')
        rows.push('')
        return rows.join('\n')
    },
    _gen_args: function (args, rows, padding) {
        var counter = 0
        args.forEach(function (arg) {
            var star = arg.args_star ? '*' : (arg.kw_star ? '**' : '')
            rows.push(padding + star + arg.name + ' (:obj:`<type>`' + (arg.default_value ? ' ,default:' + arg.default_value : '') + '): <descriptions>')
        })
    },
    selectElementContents: function (el) {
        // Copy textarea, pre, div, etc.
        if (document.body.createTextRange) {
            // IE 
            var textRange = document.body.createTextRange();
            textRange.moveToElementText(el);
            textRange.select();
            textRange.execCommand("Copy");
        } else if (window.getSelection && document.createRange) {
            // non-IE
            if (el.tagName == 'TEXTAREA') {
                el.select()
            }
            else {
                var range = document.createRange();
                range.selectNodeContents(el);
                var sel = window.getSelection();
                sel.removeAllRanges();
                sel.addRange(range);
            }
            try {
                var successful = document.execCommand('copy');
                var msg = successful ? 'successful' : 'unsuccessful';
                console.log('Copy command was ' + msg);
            } catch (err) {
                console.log('Oops, unable to copy');
            }
            //unselect
            if (window.getSelection) { window.getSelection().removeAllRanges(); }
            else if (document.selection) { document.selection.empty(); }
        }
    }, // end function selectElementContents(el) 
    make_copy_button: function (el, cel) {
        var copy_btn = document.createElement('Button');
        copy_btn.className = 'w2ui-btn'
        el.appendChild(copy_btn);
        var self = this
        copy_btn.onclick = function () {
            self.selectElementContents(cel);
        };
        if (document.queryCommandSupported("copy") || parseInt(navigator.userAgent.match(/Chrom(e|ium)\/([0-9]+)\./)[2]) >= 42) {
            // Copy works with IE 4+, Chrome 42+, Firefox 41+, Opera 29+
            copy_btn.innerHTML = "Copy to Clipboard";
        } else {
            // Select only for Safari and older Chrome, Firefox and Opera
            copy_btn.innerHTML = "Select All (then press CTRL+C to Copy)";
        }
    }
    /* Note: document.queryCommandSupported("copy") should return "true" on browsers that support copy
        but there was a bug in Chrome versions 42 to 47 that makes it return "false".  So in those
        versions of Chrome feature detection does not work!
        See https://code.google.com/p/chromium/issues/detail?id=476508
    */
}
window.gen_doc_sample = new DocSampleGenerator()
window.api_comment2w2form = function(comment,selector,options){
    // options comes from tab's data
    // this function will assign or restore options.ArgsKw
    // by the form on right-side panel
    // (2019-08-11T03:56:36+00:00)
    var apid_path = options.cmd

    // parse comment to find args and kw of this api
    var args = [],  kw = []
    var args_start_pos = -1
    var kw_start_pos = -1
    var pos = 0, c
    var end_pos = comment.length
    var args_content = ""
    var kw_content = ""
    var empty = /[\r\n\t\s]/
    var field_remarks = {}
    while (pos < end_pos){
        c = comment.charAt(pos)
        if (c == '\n'){ //seek to line start
            pos += 1
            while (empty.test(comment.charAt(pos))){pos += 1;if (pos >= end_pos) break} //skip space
            if (comment.substring(pos,pos+5).toLowerCase() == 'args:'){
                pos += 5
                while (comment.charAt(pos)!='['){pos += 1;if (pos >= end_pos) break} //seeking to first [
                if (comment.charAt(pos) == '[') {
                    args_start_pos = pos+1
                    pos += 1
                    while (comment.charAt(pos)!=']'){pos += 1;if (pos >= end_pos) break} //seeking to first ]
                    if (comment.charAt(pos) == ']') {
                        args_content = comment.substring(args_start_pos, pos)
                    }
                    else break
                }else break
            } else if (comment.substring(pos,pos+3).toLowerCase() == 'kw:'){
                pos += 3
                while (comment.charAt(pos)!='{'){pos += 1;if (pos >= end_pos) break} //seeking to first [
                if (comment.charAt(pos) == '{') {
                    kw_start_pos = pos+1
                    pos += 1
                    while (comment.charAt(pos)!='}'){pos += 1;if (pos >= end_pos) break} //seeking to first ]
                    if (comment.charAt(pos) == '}') {
                        kw_content = comment.substring(kw_start_pos, pos)
                    }
                    else break
                }else break
            } else if (comment.substring(pos,pos+1).toLowerCase() == '@'){
                pos += 1
                var start_pos = pos
                while (comment.charAt(pos)!=':'){pos += 1;if (pos >= end_pos) break} //seeking to first [
                if (comment.charAt(pos) == ':') {
                    pos += 1
                    while (comment.charAt(pos)!='\n'){pos += 1;if (pos >= end_pos) break} //seeking to first ]
                    if (comment.charAt(pos) == '\n') {
                        var content = comment.substring(start_pos, pos)
                        var p = content.indexOf(':')
                        field_remarks[content.substr(0,p).trim()] = content.substr(p+1).trim()
                        //再倒退一個，彌補被被消耗掉的換行
                        pos -= 1
                    }
                    else break
                }else break
            }
        }
        pos += 1
    }
    args_content = args_content.trim()
    if (args_content.length){
        args_content.split(',').forEach(function(item){
            var arg = []
            item.split(':').forEach(function(v){
                v = v.trim().replace(/^["']|["']$/g,'')
                if (v.length) arg.push(v)
            })
            if (arg.length) {
                if (arg[0].indexOf('*')==0){
                    var required = true
                    var field = arg[0].substr(1)
                }else{
                    var required = false
                    var field = arg[0]
                }
                args.push({
                    field:field,
                    required:required,
                    value:arg[1] || '',
                    remark:field_remarks[field]
                })
            }
        })
    }
    kw_content = kw_content.trim()
    if (kw_content.length){
        kw_content.split(',').forEach(function(item){
            var arg = []
            item.split(':').forEach(function(v){
                v = v.trim().replace(/^["']|["']$/g,'')
                if (v.length) arg.push(v)
            })
            if (arg.length) {
                if (arg[0].indexOf('*')==0){
                    var required = true
                    var field = arg[0].substr(1)
                }else{
                    var required = false
                    var field = arg[0]
                }                
                kw.push({
                    field:field,
                    required:required,
                    value:arg[1] || '',
                    remark:field_remarks[field]
                })
            }
        })
    }

    // generate w2form and merge value in existing options.ArgsKw
    var ArgsKw = {
        Args: args,
        Kw: kw
    }
    var fields = []
    var record = {}
    ArgsKw.Args.forEach(function(arg,idx){
        fields.push({name:arg.field,type:'text',required:arg.required})
        if (typeof(options.ArgsKw.Args[idx]) != 'undefined') {
            arg.value = options.ArgsKw.Args[idx].value
        }
        record[arg.field] = arg.value
    })
    ArgsKw.Kw.forEach(function(kw,idx){
        fields.push({name:kw.field,type:'text',required:kw.required})
        if (typeof(options.ArgsKw.Kw[idx]) != 'undefined') kw.value = options.ArgsKw.Kw[idx].value
        record[kw.field] = kw.value
    })
    //update  to options because the api's comment might be changed
    options.ArgsKw = ArgsKw

    // generate html of this form
    var html = ['<div class="form" style="width:100%;height:100%">']
    html.push('<div style="width:100%;height:100%;" class="w2ui-page page-0">')
    html.push('<div style="width: 100%; float: left; ">')
    html.push('<div style="padding: 3px; font-weight: bold; color: #777;">Args</div>')
    html.push('<div class="Args w2ui-group" style="height:100%;">')
    ArgsKw.Args.forEach(function(arg,idx){
        html.push('<div class="w2ui-field w2ui-span4">')
        html.push('<label>'+arg.field+'</label>')
        html.push('<div>')
        html.push('<input arg="'+idx+'" name="'+arg.field+'" value="'+arg.value+'" type="text" maxlength="100" style="width: 100%">')
        if(arg.remark) html.push('<p class="field-remark">'+arg.remark+'</p>')
        html.push('</div>')
        html.push('</div>') //end class="w2ui-field 
    })
    html.push('</div>') //end group
    html.push('</div>') //end float-left

    html.push('<div style="width:100%; float: left;">')
    html.push('<div style="padding: 3px; font-weight: bold; color: #777;">Kw</div>')
    html.push('<div class="Kw w2ui-group" style="height: 100%">')
    ArgsKw.Kw.forEach(function(kw,idx){
        html.push('<div class="w2ui-field w2ui-span4">')
        html.push('<label>'+kw.field+'</label>')
        html.push('<div>')
        html.push('<input kw="'+idx+'" name="'+kw.field+'" value="'+kw.value+'" type="text" maxlength="100" style="width: 100%">')
        html.push('</div>')
        html.push('</div>') //end class="w2ui-field 
    })
    html.push('</div>') // end group
    html.push('</div>') //end float-left

    html.push('<div style="padding: 3px; font-weight: bold; color: #777;">Stress Test Options</div>')
    html.push('<div class="Args w2ui-group" style="height:100%;">')
    html.push('<div class="w2ui-field w2ui-span4">')
    html.push('<label>Repeat</label>')
    html.push('<div>')
    html.push('<input name="repeat" value="1" type="text">')
    html.push('</div>')
    html.push('</div>') //end class="w2ui-field 
    html.push('<div class="w2ui-field w2ui-span4">')
    html.push('<label>Batch Size</label>')
    html.push('<div>')
    html.push('<input name="batchsize" value="1" type="text">')
    html.push('</div>')
    html.push('</div>') //end class="w2ui-field 
    html.push('</div>') //end group
    html.push('</div>') //end float-left

    html.push('<div class="w2ui-buttons">\
    <button class="w2ui-btn w2ui-btn-blue" style="display:none" name="call">Call</button>\
    <button class="w2ui-btn w2ui-btn-red" style="display:none" name="kill">Kill</button>\
    </div>\
    ')
    html.push('</div></div>') // end w2ui-page and .form  

    if (w2ui['auto-form']) w2ui['auto-form'].destroy()
    $(selector).html(html.join(''))
    _.defer(function(){
        //enable the w2form
        $(selector).find('.form').w2form({
            name: 'auto-form'
            ,fields:fields
            ,record:record
            ,actions: {
                kill:function(){
                    window.output_panel_manager.kill_task()
                    $(selector).find('.form .w2ui-buttons button[name="kill"]').hide()
                    $(selector).find('.form .w2ui-buttons button[name="call"]').show()
                }
                ,call: function () {
                    //reset related properties of options 
                    options.promise = null
                    options.success = null
                    options.failure = null
                    options.command = null

                    var args = []
                    for(var i=0;i<ArgsKw.Args.length;i++){
                        args.push($('.Args input[arg="'+i+'"]').val())
                    }
                    var kw = {}
                    for(var i=0;i<ArgsKw.Kw.length;i++){
                        kw[ArgsKw.Kw[i].field] = $('.Kw input[kw="'+i+'"]').val()
                    }

                    $(selector).find('.form .w2ui-buttons button[name="kill"]').show()
                    $(selector).find('.form .w2ui-buttons button[name="call"]').hide()
    
                    window.run_command(function(promise){
                        options.promise = promise
                        window.output_panel_manager.set_promise(options.promise)
                        options.promise.done(function(response){
                            options.success = response
                            options.promise = null
                            $(selector).find('.form .w2ui-buttons button[name="kill"]').hide()
                            $(selector).find('.form .w2ui-buttons button[name="call"]').show()        
                        }).fail(function(err){
                            options.failure = err
                            options.promise = null
                            $(selector).find('.form .w2ui-buttons button[name="kill"]').hide()
                            $(selector).find('.form .w2ui-buttons button[name="call"]').show()        
                        })
                        //.progress(function(_,command){
                        //    options.command = command
                        //})                     
                    },{api_id:options.cmd,args,kw})

                }
            }
        })
        //update user input
        _.defer(function(){
            // setup cancel or call buttons
            if (options.promise){
                $(selector).find('.form .w2ui-buttons button[name="kill"]').show()
            }else{
                $(selector).find('.form .w2ui-buttons button[name="call"]').show()
            }

            $(selector).find('.form .Args input').on('change',function(evt){
                var idx = parseInt(evt.currentTarget.getAttribute('arg'))
                options.ArgsKw.Args[idx].value = evt.currentTarget.value
            })
            $(selector).find('.form .Kw input').on('change',function(evt){
                var idx = parseInt(evt.currentTarget.getAttribute('kw'))
                options.ArgsKw.Kw[idx].value = evt.currentTarget.value
            })
        })
    })
}
/*
 * DocSampleGenerator end
 */
function build_toolbar() {
    //"Run" button on toolbar
    window.run_command = function (callback,options) {
        /*
         options:{
            api_id: 
            args:
            kw:
         }
         */
        if (typeof(options) == 'undefined') options = {}
        var api_id = options.api_id || $('#command-line').val().trim()        
        
        w2ui['layout'].show('preview')
        var fg_toolbar = w2ui['layout'].get('preview').toolbar
        var api_path = api_id.replace(new RegExp('^'+sdk.tree.treeName+'\.'),'')

        var args = options.args
        if (!args){
            args = []
            $('#command-args').val().trim().split(',').forEach(function(arg){
                args.push(arg)
            })    
        }

        var kw = options.kw
        if (!kw){
            $('#command-kw').val().trim().split(',').forEach(function(kv){
            
                var p = kv.indexOf('=')
                if (p == -1) return
                if (kw === null) kw = {}
                kw[kv.substr(0,p).trim()] = kv.substr(p+1).trim()
            })    
        }
        var promise = sdk.tree.call(api_path,args,kw)
        callback(promise)
        /*
        else{
            promise.done(function (response) {
                w2ui.toolbar.enable('runcmd')
                fg_toolbar.disable('cancelcmd')
                fg_toolbar.disable('tobgcmd')
                command_handler.default_handler(response)
                
            }).progress(function (response,command) {
                fg_toolbar.enable('cancelcmd')
                fg_toolbar.enable('tobgcmd')
                command_handler.default_handler(response)
                window.cancel_task = function () {
                    return sdk.tree.cancel({id:command.get('id'), name:command.get('name')})
                }
                if (callback) callback(response)
            }).fail(function(retcode,err){
                command_handler.default_errhandler(retcode,err)
            })    
        }*/
    }
    window.send_to_background = function (task_id) {
        sdk.task.to_background(task_id).done(function (success, errmsg) {
            if (success) {
                setTimeout(function () { list_tasks() }, 100)
                w2ui['layout'].content('preview', '')
            }
            else {
                show_message(errmsg)
            }
        })
    }
    var repeat_running = false
    window.repeat_run = function (ele) {
        if (repeat_running) {
            repeat_running = false
            /*
            $('#repeat-run-btn')[0].className=''
            $('#repeat-run-btn')[0].innerText='Repeat Run'
            */
            ele.innerText = 'Go'
        }
        else {

            var value = $('#command-line').val().trim()
            if (!value) return

            repeat_running = ele

            //$('#repeat-run-btn')[0].className='red'
            ele.innerText = 'Stop'

            var command = Command.from_line(value)
            var handler = command_handler[command.content.cmd]
            if (!handler) handler = command_handler.default_handler
            var run = function () {
                run_command(function (response) {
                    var interval = parseInt($('#repeat-run-interval').val()) * 1000
                    if (repeat_running) setTimeout(function () { run() }, interval)
                })
            }
            run()
        }
    }

    window.last_watching_task_id = 0;

    var do_list_tasks = function (promise_of_task_command) {
        // promise_of_task_command is the return value of sdk.task.list()
        w2ui['layout'].show('right')
        promise_of_task_command.fail(function (errmsg) {
            console.warn(errmsg)
        })
        promise_of_task_command.done(function (taskdata_dict) {
            var running_tasks = []
            var cached_tasks = []
            for (var task_id in taskdata_dict) {
                var taskdata = taskdata_dict[task_id] //let
                //console.log(taskdata)
                var task_name = taskdata.name ? '(' + taskdata.name + ')' : ''
                if (taskdata.alive) {
                    running_tasks.push({ id: taskdata.id, text: task_name + taskdata.command_line })
                }
                else {
                    cached_tasks.push({ id: taskdata.id, text: task_name + ('' + taskdata.state_name + ': ' + taskdata.command_line) })
                }
            }

            if (w2ui['tasks-list-bar']) w2ui['tasks-list-bar'].destroy()
            var nodes = []
            if (running_tasks.length) {
                nodes.push({
                    id: 'running',
                    text: 'Running(' + running_tasks.length + ')',
                    img: null, expanded: true, group: true,
                    nodes: running_tasks
                })
            }
            if (cached_tasks.length) {
                nodes.push({
                    id: 'cached',
                    text: 'Cached(' + cached_tasks.length + ')',
                    img: null, expanded: true, group: true,
                    nodes: cached_tasks
                })
            }
            $().w2sidebar({
                name: 'tasks-list-bar',
                topHTML: 'Total count:' + (running_tasks.length + cached_tasks.length),
                nodes: nodes
            });

            w2ui['right_inner_layout'].content('main', w2ui['tasks-list-bar'])
            w2ui['right_inner_layout'].content('preview', '')

            if (running_tasks.length + cached_tasks.length == 0) return

            //
            //  enable watching
            //

            var bg_toolbar = w2ui['right_inner_layout'].get('preview').toolbar
            var watching_finished = function () {
                w2ui['tasks-list-bar'].unselect(window.last_watching_task_id)
                window.last_watching_task_id = 0
                bg_toolbar.disable('bg_cancelcmd')
            }
            var watch_handler = function (command_result) {
                $('#refreshing_mark').html('*')
                setTimeout(function () { $('#refreshing_mark').html('') }, 250)
                if (command_result) {
                    var content = '<pre id="cmd_progress">' + JSON.stringify(command_result, null, 2) + '</pre>'
                    if (command_result.retcode === null) {
                        // this task is in progress
                        bg_toolbar.enable('bg_cancelcmd')
                    }
                    else {
                        // this task is completed
                        watching_finished()
                    }
                    if (command_result.background) {
                        bg_toolbar.disable('bg_bgcmd')
                    }
                    else {
                        bg_toolbar.enable('bg_bgcmd')
                    }
                    w2ui['right_inner_layout'].content('preview', content)
                }
                else {
                    //this task has reaches its ttl, been removed
                    //ensure it was unchecked, because it has been removed from watching list by sdk 
                    watching_finished()
                }
            }
            var watch_err_handler = function (error_message) {
                w2ui['right_inner_layout'].content('preview', '<pre>' + JSON.stringify(error_message, null, 2) + '</pre>')
                watching_finished()
            }
            w2ui['tasks-list-bar'].on('click', function (evt) {

                var task_id = evt.target
                var alive = false
                running_tasks.some(function (item) {
                    if (item.id == task_id) { alive = true; return true }
                })

                if (alive) {
                    if (window.last_watching_task_id == task_id) {
                        //toggle, to unwatch current watching task
                        sdk.task.unwatch(task_id).done(function () {
                            watching_finished()
                        })
                        return
                    }
                    // unwatch watching task
                    if (window.last_watching_task_id) {
                        // unwatch current watching task
                        sdk.task.unwatch(window.last_watching_task_id).done(function (success, payload) {
                            if (!success) {
                                console.warn(payload)
                            }
                        })
                        w2ui['tasks-list-bar'].unselect(window.last_watching_task_id)
                        window.last_watching_task_id = 0
                    }
                    window.last_watching_task_id = task_id
                    sdk.task.watch(task_id).progress(watch_handler).fail(watch_err_handler)
                }
                else {
                    // retrive a cached background task

                    // unwatch watching task
                    if (window.last_watching_task_id) {
                        // unwatch current watching task
                        sdk.task.unwatch(window.last_watching_task_id).done(function (success, payload) {
                            if (!success) {
                                console.warn(payload)
                            }
                        })
                        w2ui['tasks-list-bar'].unselect(window.last_watching_task_id)
                        window.last_watching_task_id = 0
                    }

                    w2ui['tasks-list-bar'].unselect(task_id)
                    sdk.task.watch(task_id).progress(watch_handler).fail(watch_err_handler)
                }
            })

            if (running_tasks.length) {
                // [score, task_id]
                var candidates = [0, null]
                var taskdata
                for (var i = running_tasks.length - 1; i >= 0; --i) {
                    taskdata = taskdata_dict[running_tasks[i].id]
                    if (taskdata.background && taskdata.alive) {
                        candidates = [3, taskdata.id]
                        break
                    }
                    else if (taskdata.background) {
                        candidates = [2, taskdata.id]
                    }
                }
                // auto-click on candidate taskdata of highest score
                /*
                if (candidates[0]>0) {
                    w2ui['tasks-list-bar'].click(candidates[1])
                    w2ui['tasks-list-bar'].select(candidates[1])
                }
                */
            }
        })
    }

    window.list_tasks = function () {
        // This is a wrapper of do_list_tasks().
        // For ensuring that there is no watching task before watching another task.
        if (window.last_watching_task_id) {
            window.last_watching_task_id = 0
            sdk.task.unwatch(window.last_watching_task_id).done(function () {
                do_list_tasks(sdk.task.list())
            })
        }
        else {
            do_list_tasks(sdk.task.list())
        }
    }
    window.search_tasks = function (keyword, search_scope) {
        do_list_tasks(sdk.task.search(keyword, search_scope))
    }

    //generate pbtype_items
    /*
    $('#toolbar').w2toolbar({
        name: 'toolbar',
        items: [
            { type: 'menu', id: 'savedcmd', text: 'Branche:', icon: 'fa fa-list', items: [] },
            {
                type: 'html', id: 'cmd',
                html: function (item) {
                    var html =
                        '<div style="padding: 3px 10px;">' +
                        '    <input style="width:150px" id="command-line" placeholder="Tree.UnitTest.Hello" value="' + (item.value || '') + '"/>' +
                        '    <input style="width:100px" id="command-args" placeholder="arg1,arg2,..." value=""/>' +
                        '    <input style="width:100px" id="command-kw" placeholder="key=value,..." value=""/>' +
                        '</div>';
                    return html;
                }
            },
            { type: 'menu', id: 'pb-message', text: 'Protobuf', icon: 'fa fa-list', items:[] },
            {
                type: 'button', id: 'runcmd', icon: 'fa fa-play', text: 'Call', css:'border:solid 1px black',
                counter: 0,
                tooltip: function (item) {
                    return 'Run the command in server';
                },
                onClick: function (event) {
                    //event.item.counter++;

                    var value = $('#command-line').val().trim()
                    if (!value) return

                    //auto stop repeat-running 
                    if (repeat_running) repeat_run() //toggle it to stop
                    run_command(function (promise) {
                        // auto click "List of Tasks'" button for background task
                        promise.done(function(response){
                            if (response.background) {
                                w2ui['toolbar'].enable('cancelcmd')
                                setTimeout(function () { list_tasks() }, 10)
                            }
                            else {
                                w2ui['toolbar'].enable('tobgcmd')
                            }    
                        })
                    })
                }
            },
            {
                type: 'drop', id: 'item4', text: 'Repeat', icon: 'fa fa-play-circle',
                html: '<div style="padding: 10px; line-height: 1.5">' +
                    'once per <input style="text-align:center; width:30px" id="repeat-run-interval" value="1"> seconds.' +
                    '<button class="w2ui-btn" onclick="repeat_run(this)" style="width:40px;padding:3px 5px">Go</button></div>'
            },
            { type: 'break' },
            {
                type: 'button', id: 'savecmd', icon: 'fa fa-plus', text: 'Save',
                tooltip: function (item) {
                    return 'Save the command';
                },
                onClick: function (event) {
                    //event.item.counter++;
                    var value = $('#command-line').val().trim()
                    if (!value) return
                    var command = Command.from_line(sdk.metadata.runner_name + '.root.playground.gui_settings_change add_command_line "' + value + '"')
                    sdk.send_command(command).done(function (response) {
                        console.log(response)
                        if (response.retcode == 0) {
                            saved_command_lines.push(value)
                            update_saved_command_lines()
                            show_message('Command saved')
                        }
                        else {
                            show_message(response.stderr)
                        }
                    })
                }
            },
            {
                type: 'button', id: 'removecmd', icon: 'fa fa-times', text: 'Remove',
                counter: 0,
                tooltip: function (item) {
                    return 'Remove the command if it is saved';
                },
                onClick: function (event) {
                    var value = $('#command-line').val().trim()
                    if (!value) return
                    var idx = saved_command_lines.indexOf(value)
                    if (idx == -1) return
                    var command = Command.from_line(sdk.metadata.runner_name + '.root.playground.gui_settings_change del_command_line "' + value + '"')
                    sdk.send_command(command).done(function (response) {
                        if (response.retcode == 0) {
                            saved_command_lines.splice(idx, 1)
                            update_saved_command_lines()
                            show_message('Command removed')
                        }
                        else {
                            show_message(response.stderr)
                        }
                    })
                }
            },
            { type: 'break' },
            {
                type: 'button', id: 'tasklist', icon: 'fa fa-tasks', text: 'Tasks',
                counter: 0,
                tooltip: function (item) {
                    return 'List alive tasks';
                },
                onClick: function (event) {
                    //event.item.counter++;
                    list_tasks()
                }
            },
            {
                type: 'drop', id: 'tasklistbyname', text: 'Search', icon: 'fa fa-play-circle',
                tooltip: function (item) {
                    return 'Search alive tasks';
                },
                html: '<div style="padding:5px; line-height: 1.5" class="searchtaskbox">' +
                    '   <div><input name="task_name">' +
                    '       <button class="w2ui-btn" onclick="search_tasks($(\'.searchtaskbox input[name=task_name]\').val(),$(\'.searchtaskbox input[name=scope]:checked\').val())" style="width:40px;padding:3px 5px">Go</button>' +
                    '   </div>' +
                    '   <div style="padding:5px; line-height: 1.5">' +
                    '       <input style="margin-right:5px" type="radio" value="cmd" name="scope">Search command name</br>' +
                    '       <input style="margin-right:5px" type="radio" value="name" name="scope">Search task name<br/>' +
                    '       <input style="margin-right:5px" checked type="radio" value="*" name="scope">Both' +
                    '   </div>' +
                    '</div>'
            },
            { type: 'spacer' },
            {
                type: 'button', id: 'logout', icon: 'fa fa-user-circle', text: 'Logout',
                counter: 0,
                tooltip: function (item) {
                    return 'Logout this browser';
                },
                onClick: function (event) {
                    w2confirm('Logout ?').yes(function () {
                        var logout_url = '/playground/logout'
                        sdk.logout(logout_url).done(function(){
                            console.log('logout success')
                            location.reload()
                        })
                        
                    })
                }
            },
        ]
    })
    //auto run if command-line has ENTRY
    $('#command-line').keypress(function (evt) {
        if (evt.keyCode != 13) return // not ENTRY
        run_command()
    })

    w2ui['toolbar'].on('click', function (evt) {
        var name_subname = evt.target.split(':')
        if (name_subname.length == 1) return
        switch (name_subname[0]) {
            case 'savedcmd':
                var line = saved_command_lines[parseInt(name_subname[1])]
                $('#command-line').val(line)
                break
        }
    })
    */
}


function build_sidebar(tree_layout){
    if (w2ui['sidebar']) {
        w2ui['sidebar'].destroy()
    }
    var get_nodes = function(){
        var nodes = []
        var branches = []
        for (var treebranchname in tree_layout){
            /*{
                Note: "treebranchname" includes TreeName
                Structure:
                    Tree.$chat:{          <<== treebranchname
                        funcName:{
                            Comments:""
                        }
                    }
            }*/
            var callnodes = []
            var funcnames = []
            for (var funcname in tree_layout[treebranchname]){
                funcnames.push(funcname)
            }
            funcnames.sort()
            funcnames.forEach(function(funcname){
                var apiinfo = tree_layout[treebranchname][funcname]
                callnodes.push({
                    id:treebranchname+'.'+funcname
                    ,text:funcname
                    ,apiinfo:apiinfo
                })
            })
            branches.push({
                id:treebranchname
                ,text:treebranchname
                ,group:true
                ,expanded:true
                ,nodes:callnodes
                ,count:callnodes.length
            })
        }
        nodes.push(
            { id: 'statetree', text: 'Branches', group: true, nodes: branches, expanded: true },
        )
        return nodes
    }

    var tree_selector = ['<div class="w2ui-field" id="tree-selector">Tree:<select>']
    window.treedata.forEach(function(tree,i){
        var selected = tree.selected ? ' selected' : ''
        tree_selector.push('<option value="'+i+'"'+selected+'>'+tree.name+'</option>')
    })
    tree_selector.push('</select></div>')

    var data = {
        topHTML: '<div style="padding: 10px 5px; border-bottom: 1px solid silver">'+tree_selector.join('')+'</div>',
        bottomHTML: '',
        name: 'sidebar',
        flatButton: true,
        img: null,
        nodes:get_nodes(),
        onFlat: function (event) {
            $(w2ui['layout'].el('left')).css('width', (event.goFlat ? '30px' : '200px'));
            w2ui['layout'].sizeTo('left', event.goFlat ? 30 : 200)
        },
        onClick: function (event) {
            console.log(event)
            var id = event.target
            if (id.indexOf(sdk.tree.treeName) == 0){
                //click on a api leafs
                var api_path = id
                $('#command-line').val(api_path)
                //if shift, alt, ctrl is pressed, enforce to open a new tab, otherwise find an existing one before creating a new tab
                var force2add = event.originalEvent.shiftKey || event.originalEvent.metaKey || event.originalEvent.ctrlKey
                if (!force2add){
                    for (var tab_id in window.node_tabs_manager.tab_data){
                        if (window.node_tabs_manager.tab_data[tab_id].cmd == api_path){
                            _.defer(function(){window.node_tabs_manager.tabs.click(tab_id)})
                            return
                        }
                    }
                }
                window.node_tabs_manager.add({
                    apiinfo:w2ui['sidebar'].get(api_path).apiinfo,
                    cmd: api_path,
                    name: w2ui['sidebar'].get(api_path).text,
                    ArgsKw:{ //placeholder for been assigned later
                        Args:[],
                        Kw:{}
                    },
                    promise: null,
                    success: null,
                    failure: null,
                    command: null
                })
            } else if (id.indexOf('playground') == 0) {
                switch (id) {
                    case 'playground-daemons':
                        var command = Command.from_line(sdk.metadata.runner_name + '.root.playground.get_daemons')
                        sdk.send_command(command).done(function(response){
                            var records = []
                            var recid = 0
                            for (var name in response.stdout){
                                var value = response.stdout[name]
                                recid += 1
                                records.push({
                                    'recid':recid,
                                    'name':name,
                                    'type':value.type,
                                    'port':value.port,
                                    'enable': value.enable,
                                    'acls':value.acls.join(','),

                                })
                            }
                            records.sort(function(a,b){return (a.port > b.port ? 1 : (a.port < b.port ? -1 : 0))})
                            var content = '<div id="daemons-data" style="width:100%;height:100%"></div>'
                            w2ui['main_layout'].content('right',content)
                            w2ui['main_layout'].show('right')
                            if (w2ui['daemons-data']) w2ui['daemons-data'].destroy()
                            $('#daemons-data').w2grid({
                                name:'daemons-data',
                                show: {
                                    toolbar:false //disable temporary
                                },
                                columns: [                
                                    { field: 'name', caption: 'Name',size:'20%'},
                                    { field: 'type', caption: 'Type',size:'20%'},
                                    { field: 'port', caption: 'Port',size:'15%'},
                                    { field: 'acls', caption: 'ACLs',size:'35%'},
                                    { field: 'enable', caption: 'Enable',size:'10%'},
                                ],
                                records:records
                            })
                            setTimeout(function(){
                                w2ui['daemons-data'].refresh()
                            },100)
                        })
                        
                        break
                    case 'playground-settings':
                        if (playground_settings_form) playground_settings_form.destroy()
                        w2ui['layout'].content('main', '<div id="playground-settings"></div>')
                        playground_settings_form = $('#playground-settings').w2form({
                            name: 'playground-settings',
                            header: 'Play Ground Settings',
                            fields: [
                                {
                                    field: 'Node Path', type: 'radio', required: false,
                                    options: {
                                        items: [
                                            { id: 1, text: 'absolute, starts from root.' },
                                            { id: 2, text: 'relative, starts from nodes.' },
                                        ]
                                    }
                                }
                            ],
                            actions: {
                                Save: function () {
                                }
                            }
                        })
                        break
                }
            }
            else if (id.indexOf('url-') == 0) {
                open(event.node.url)
            }
            else {
                var item = nodes[id]
                window.node_tabs_manager.add(id,item)
            }
        }
    }
    w2ui['layout'].content('left', $().w2sidebar(data))
    w2ui['layout'].show('left')
    _.defer(function(){
        $('#tree-selector select').on('change',function(evt){
            var selected_idx = parseInt($(evt.currentTarget).val())
            for (var i=0;i<window.treedata.length;i++){
                var tree = window.treedata[i]
                tree.selected = (i == selected_idx) ? true : false
            }
            var tree = window.treedata[selected_idx]
            sdk.useTree(tree.name, tree.url)
            console.log(window.treedata)
        })    
    })
}

/*
function build_layout() {
    var command = Command.from_line(sdk.metadata.runner_name + '.root.playground.gui_initdata')
    window.utility.loading('Loading')
    sdk.send_command(command).done(function (command_result) {
        window.utility.loading(false)
        if (command_result.retcode != 0) {
            w2ui['layout'].content('preview', JSON.stringify(command_result, null, 2))
        }
        else {
            if (sdk._sidebar) sdk._sidebar.destroy()
            var result = command_result.stdout
            saved_command_lines = result.gui_settings.saved_command_lines
            update_saved_command_lines()
            

            var nodes = []
            if (result.documents.links.length) {
                var document_links = []
                result.documents.links.forEach(function (item, idx) {
                    document_links.push({ id: 'url-doc-' + idx, text: item[0], url: item[1] })
                })
                nodes.push({ id: 'documents', text: 'Documents', group: true, expanded: false, nodes: document_links })
            }
            // if id starts with 'url-', it will be opened as an external link
            var misc_links = [
                { id:'url-leafnodes', text: 'Leaf Nodes', url: '/websdk/leafnodes.html' },
                { id:'playground-daemons', text:'Deamons'}
            ]
            nodes.push({ id: 'misc', text: 'Misc', group: true, expanded: false, nodes: misc_links})
            / * not implemented, temporary disabled
            {id:'playground',text:'Play Ground',group:true, expanded:false,nodes:[
                {id:'playground-settings',text:'Settings'}
            ]}
            * /
            //format the result
            
            var gen = function (node, a_list, id_prefix) {
                if (node.children && node.children.length) {
                    var sorted = []
                    node.children.forEach(function (item) {
                        sorted.push(item)
                    })
                    sorted.sort(function (a, b) { return (a.name > b.name ? 1 : (a.name == b.name ? 0 : -1)) })
                    sorted.forEach(function (item) {
                        var item_data = {id: id_prefix + item.name, text: item.name }
                        nodes[item_data.id] = item
                        // append "path" to node's metadata
                        item.path = id_prefix + item.name, 
                        a_list.push(item_data)
                        if (item.children && item.children.length) {
                            item_data.nodes = []
                            //item_data.group = true
                            item_data.expanded = true
                            item_data.img = 'fa fa-cubes'
                            gen(item, item_data.nodes, item_data.id + '.')
                        }
                        else {
                            item_data.img = 'fa fa-cube'
                        }
                    })
                }
            }
            var statetree_nodes = []
            var playground_settings_form;
            gen(result.hierarchy, statetree_nodes, 'root.')
            nodes.push(
                { id: 'statetree', text: 'State Tree', group: true, nodes: statetree_nodes, expanded: true },
            )

            var data = {
                topHTML: '<div style="padding: 10px 5px; border-bottom: 1px solid silver">&nbsp;</div>',
                bottomHTML: '',
                name: 'sidebar',
                flatButton: true,
                img: null,
                nodes:nodes,
                onFlat: function (event) {
                    $(w2ui['layout'].el('left')).css('width', (event.goFlat ? '30px' : '200px'));
                    w2ui['layout'].sizeTo('left', event.goFlat ? 30 : 200)
                },
                onClick: function (event) {
                    var id = event.target
                    if (id.indexOf('playground') == 0) {
                        switch (id) {
                            case 'playground-daemons':
                                var command = Command.from_line(sdk.metadata.runner_name + '.root.playground.get_daemons')
                                sdk.send_command(command).done(function(response){
                                    var records = []
                                    var recid = 0
                                    for (var name in response.stdout){
                                        var value = response.stdout[name]
                                        recid += 1
                                        records.push({
                                            'recid':recid,
                                            'name':name,
                                            'type':value.type,
                                            'port':value.port,
                                            'enable': value.enable,
                                            'acls':value.acls.join(','),

                                        })
                                    }
                                    records.sort(function(a,b){return (a.port > b.port ? 1 : (a.port < b.port ? -1 : 0))})
                                    var content = '<div id="daemons-data" style="width:100%;height:100%"></div>'
                                    w2ui['main_layout'].content('right',content)
                                    w2ui['main_layout'].show('right')
                                    if (w2ui['daemons-data']) w2ui['daemons-data'].destroy()
                                    $('#daemons-data').w2grid({
                                        name:'daemons-data',
                                        show: {
                                            toolbar:false //disable temporary
                                        },
                                        columns: [                
                                            { field: 'name', caption: 'Name',size:'20%'},
                                            { field: 'type', caption: 'Type',size:'20%'},
                                            { field: 'port', caption: 'Port',size:'15%'},
                                            { field: 'acls', caption: 'ACLs',size:'35%'},
                                            { field: 'enable', caption: 'Enable',size:'10%'},
                                        ],
                                        records:records
                                    })
                                    setTimeout(function(){
                                        w2ui['daemons-data'].refresh()
                                    },100)
                                })
                                
                                break
                            case 'playground-settings':
                                if (playground_settings_form) playground_settings_form.destroy()
                                w2ui['layout'].content('main', '<div id="playground-settings"></div>')
                                playground_settings_form = $('#playground-settings').w2form({
                                    name: 'playground-settings',
                                    header: 'Play Ground Settings',
                                    fields: [
                                        {
                                            field: 'Node Path', type: 'radio', required: false,
                                            options: {
                                                items: [
                                                    { id: 1, text: 'absolute, starts from root.' },
                                                    { id: 2, text: 'relative, starts from nodes.' },
                                                ]
                                            }
                                        }
                                    ],
                                    actions: {
                                        Save: function () {
                                        }
                                    }
                                })
                                break
                        }
                    }
                    else if (id.indexOf('url-') == 0) {
                        open(event.node.url)
                    }
                    else {
                        var item = nodes[id]
                        window.node_tabs_manager.add(id,item)
                    }
                }
            }
            sdk._sidebar = $().w2sidebar(data)
            w2ui['layout'].content('left', sdk._sidebar)
        }
    })
    sdk.channel.event.add_listener(function (event_obj) {
        var recid = new Date().getTime()/1000//let
        var record = {recid:recid, payload:event_obj.payload ? JSON.stringify(event_obj.payload,null,2) : '', time:event_obj.ts, name:event_obj.name, source:event_obj.source}//let
        w2ui['events-table'].add(record,true)
    })
}
*/