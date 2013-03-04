#!/usr/bin/env ruby
# encoding: utf-8

require 'base64'
require 'msgpack'

def encode_string(data)
  "#{Base64.strict_encode64(data.to_msgpack)}\n" rescue ""
end

writer = File.open('failed_events.log', 'a')
writer.sync = true

for n in 0..10000
  event = {
    :test => {
      :id => n
    }
  }
  writer.write encode_string(event)
end

writer.close()
