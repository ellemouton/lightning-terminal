/* eslint-disable */
var proto = { looprpc: {} };

// source: swapserverrpc/reservation.proto
/**
 * @fileoverview
 * @enhanceable
 * @suppress {missingRequire} reports error on implicit type usages.
 * @suppress {messageConventions} JS Compiler reports an error if a variable or
 *     field starts with 'MSG_' and isn't a translatable message.
 * @public
 */
// GENERATED CODE -- DO NOT EDIT!
/* eslint-disable */
// @ts-nocheck

var jspb = require('google-protobuf');
var goog = jspb;
var global =
    (typeof globalThis !== 'undefined' && globalThis) ||
    (typeof window !== 'undefined' && window) ||
    (typeof global !== 'undefined' && global) ||
    (typeof self !== 'undefined' && self) ||
    (function () { return this; }).call(null) ||
    Function('return this')();

goog.exportSymbol('proto.looprpc.ReservationNotificationRequest', null, global);
goog.exportSymbol('proto.looprpc.ReservationProtocolVersion', null, global);
goog.exportSymbol('proto.looprpc.ServerOpenReservationRequest', null, global);
goog.exportSymbol('proto.looprpc.ServerOpenReservationResponse', null, global);
goog.exportSymbol('proto.looprpc.ServerReservationNotification', null, global);
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.looprpc.ReservationNotificationRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.looprpc.ReservationNotificationRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.looprpc.ReservationNotificationRequest.displayName = 'proto.looprpc.ReservationNotificationRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.looprpc.ServerReservationNotification = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.looprpc.ServerReservationNotification, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.looprpc.ServerReservationNotification.displayName = 'proto.looprpc.ServerReservationNotification';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.looprpc.ServerOpenReservationRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.looprpc.ServerOpenReservationRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.looprpc.ServerOpenReservationRequest.displayName = 'proto.looprpc.ServerOpenReservationRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.looprpc.ServerOpenReservationResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.looprpc.ServerOpenReservationResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.looprpc.ServerOpenReservationResponse.displayName = 'proto.looprpc.ServerOpenReservationResponse';
}



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.looprpc.ReservationNotificationRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.looprpc.ReservationNotificationRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.looprpc.ReservationNotificationRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.looprpc.ReservationNotificationRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    protocolVersion: jspb.Message.getFieldWithDefault(msg, 1, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.looprpc.ReservationNotificationRequest}
 */
proto.looprpc.ReservationNotificationRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.looprpc.ReservationNotificationRequest;
  return proto.looprpc.ReservationNotificationRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.looprpc.ReservationNotificationRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.looprpc.ReservationNotificationRequest}
 */
proto.looprpc.ReservationNotificationRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.looprpc.ReservationProtocolVersion} */ (reader.readEnum());
      msg.setProtocolVersion(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.looprpc.ReservationNotificationRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.looprpc.ReservationNotificationRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.looprpc.ReservationNotificationRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.looprpc.ReservationNotificationRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getProtocolVersion();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
};


/**
 * optional ReservationProtocolVersion protocol_version = 1;
 * @return {!proto.looprpc.ReservationProtocolVersion}
 */
proto.looprpc.ReservationNotificationRequest.prototype.getProtocolVersion = function() {
  return /** @type {!proto.looprpc.ReservationProtocolVersion} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.looprpc.ReservationProtocolVersion} value
 * @return {!proto.looprpc.ReservationNotificationRequest} returns this
 */
proto.looprpc.ReservationNotificationRequest.prototype.setProtocolVersion = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.looprpc.ServerReservationNotification.prototype.toObject = function(opt_includeInstance) {
  return proto.looprpc.ServerReservationNotification.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.looprpc.ServerReservationNotification} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.looprpc.ServerReservationNotification.toObject = function(includeInstance, msg) {
  var f, obj = {
    reservationId: msg.getReservationId_asB64(),
    value: jspb.Message.getFieldWithDefault(msg, 2, "0"),
    serverKey: msg.getServerKey_asB64(),
    expiry: jspb.Message.getFieldWithDefault(msg, 4, 0),
    protocolVersion: jspb.Message.getFieldWithDefault(msg, 5, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.looprpc.ServerReservationNotification}
 */
proto.looprpc.ServerReservationNotification.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.looprpc.ServerReservationNotification;
  return proto.looprpc.ServerReservationNotification.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.looprpc.ServerReservationNotification} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.looprpc.ServerReservationNotification}
 */
proto.looprpc.ServerReservationNotification.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setReservationId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readUint64String());
      msg.setValue(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setServerKey(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setExpiry(value);
      break;
    case 5:
      var value = /** @type {!proto.looprpc.ReservationProtocolVersion} */ (reader.readEnum());
      msg.setProtocolVersion(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.looprpc.ServerReservationNotification.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.looprpc.ServerReservationNotification.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.looprpc.ServerReservationNotification} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.looprpc.ServerReservationNotification.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getReservationId_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getValue();
  if (parseInt(f, 10) !== 0) {
    writer.writeUint64String(
      2,
      f
    );
  }
  f = message.getServerKey_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
  f = message.getExpiry();
  if (f !== 0) {
    writer.writeUint32(
      4,
      f
    );
  }
  f = message.getProtocolVersion();
  if (f !== 0.0) {
    writer.writeEnum(
      5,
      f
    );
  }
};


/**
 * optional bytes reservation_id = 1;
 * @return {!(string|Uint8Array)}
 */
proto.looprpc.ServerReservationNotification.prototype.getReservationId = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes reservation_id = 1;
 * This is a type-conversion wrapper around `getReservationId()`
 * @return {string}
 */
proto.looprpc.ServerReservationNotification.prototype.getReservationId_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getReservationId()));
};


/**
 * optional bytes reservation_id = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getReservationId()`
 * @return {!Uint8Array}
 */
proto.looprpc.ServerReservationNotification.prototype.getReservationId_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getReservationId()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.looprpc.ServerReservationNotification} returns this
 */
proto.looprpc.ServerReservationNotification.prototype.setReservationId = function(value) {
  return jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional uint64 value = 2;
 * @return {string}
 */
proto.looprpc.ServerReservationNotification.prototype.getValue = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, "0"));
};


/**
 * @param {string} value
 * @return {!proto.looprpc.ServerReservationNotification} returns this
 */
proto.looprpc.ServerReservationNotification.prototype.setValue = function(value) {
  return jspb.Message.setProto3StringIntField(this, 2, value);
};


/**
 * optional bytes server_key = 3;
 * @return {!(string|Uint8Array)}
 */
proto.looprpc.ServerReservationNotification.prototype.getServerKey = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes server_key = 3;
 * This is a type-conversion wrapper around `getServerKey()`
 * @return {string}
 */
proto.looprpc.ServerReservationNotification.prototype.getServerKey_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getServerKey()));
};


/**
 * optional bytes server_key = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getServerKey()`
 * @return {!Uint8Array}
 */
proto.looprpc.ServerReservationNotification.prototype.getServerKey_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getServerKey()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.looprpc.ServerReservationNotification} returns this
 */
proto.looprpc.ServerReservationNotification.prototype.setServerKey = function(value) {
  return jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional uint32 expiry = 4;
 * @return {number}
 */
proto.looprpc.ServerReservationNotification.prototype.getExpiry = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.looprpc.ServerReservationNotification} returns this
 */
proto.looprpc.ServerReservationNotification.prototype.setExpiry = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional ReservationProtocolVersion protocol_version = 5;
 * @return {!proto.looprpc.ReservationProtocolVersion}
 */
proto.looprpc.ServerReservationNotification.prototype.getProtocolVersion = function() {
  return /** @type {!proto.looprpc.ReservationProtocolVersion} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.looprpc.ReservationProtocolVersion} value
 * @return {!proto.looprpc.ServerReservationNotification} returns this
 */
proto.looprpc.ServerReservationNotification.prototype.setProtocolVersion = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.looprpc.ServerOpenReservationRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.looprpc.ServerOpenReservationRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.looprpc.ServerOpenReservationRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.looprpc.ServerOpenReservationRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    reservationId: msg.getReservationId_asB64(),
    clientKey: msg.getClientKey_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.looprpc.ServerOpenReservationRequest}
 */
proto.looprpc.ServerOpenReservationRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.looprpc.ServerOpenReservationRequest;
  return proto.looprpc.ServerOpenReservationRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.looprpc.ServerOpenReservationRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.looprpc.ServerOpenReservationRequest}
 */
proto.looprpc.ServerOpenReservationRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setReservationId(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setClientKey(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.looprpc.ServerOpenReservationRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.looprpc.ServerOpenReservationRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.looprpc.ServerOpenReservationRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.looprpc.ServerOpenReservationRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getReservationId_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getClientKey_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
};


/**
 * optional bytes reservation_id = 1;
 * @return {!(string|Uint8Array)}
 */
proto.looprpc.ServerOpenReservationRequest.prototype.getReservationId = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes reservation_id = 1;
 * This is a type-conversion wrapper around `getReservationId()`
 * @return {string}
 */
proto.looprpc.ServerOpenReservationRequest.prototype.getReservationId_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getReservationId()));
};


/**
 * optional bytes reservation_id = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getReservationId()`
 * @return {!Uint8Array}
 */
proto.looprpc.ServerOpenReservationRequest.prototype.getReservationId_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getReservationId()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.looprpc.ServerOpenReservationRequest} returns this
 */
proto.looprpc.ServerOpenReservationRequest.prototype.setReservationId = function(value) {
  return jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional bytes client_key = 2;
 * @return {!(string|Uint8Array)}
 */
proto.looprpc.ServerOpenReservationRequest.prototype.getClientKey = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes client_key = 2;
 * This is a type-conversion wrapper around `getClientKey()`
 * @return {string}
 */
proto.looprpc.ServerOpenReservationRequest.prototype.getClientKey_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getClientKey()));
};


/**
 * optional bytes client_key = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getClientKey()`
 * @return {!Uint8Array}
 */
proto.looprpc.ServerOpenReservationRequest.prototype.getClientKey_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getClientKey()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.looprpc.ServerOpenReservationRequest} returns this
 */
proto.looprpc.ServerOpenReservationRequest.prototype.setClientKey = function(value) {
  return jspb.Message.setProto3BytesField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.looprpc.ServerOpenReservationResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.looprpc.ServerOpenReservationResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.looprpc.ServerOpenReservationResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.looprpc.ServerOpenReservationResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.looprpc.ServerOpenReservationResponse}
 */
proto.looprpc.ServerOpenReservationResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.looprpc.ServerOpenReservationResponse;
  return proto.looprpc.ServerOpenReservationResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.looprpc.ServerOpenReservationResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.looprpc.ServerOpenReservationResponse}
 */
proto.looprpc.ServerOpenReservationResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.looprpc.ServerOpenReservationResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.looprpc.ServerOpenReservationResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.looprpc.ServerOpenReservationResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.looprpc.ServerOpenReservationResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};


/**
 * @enum {number}
 */
proto.looprpc.ReservationProtocolVersion = {
  RESERVATION_NONE: 0,
  RESERVATION_SERVER_NOTIFY: 1
};

goog.object.extend(exports, proto.looprpc);
