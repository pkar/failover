#!/usr/bin/env ruby
# encoding: utf-8

require 'base64'
require 'json'

def encode_string(data)
  "#{Base64.strict_encode64(data.to_json)}\n" rescue ""
end

writer = File.open('failed_events.log', 'a')
writer.sync = true

for n in 0..1000
  event = {
    :test => {
      :id => n
    }
  }
  writer.write encode_string(event)
end

writer.close()
