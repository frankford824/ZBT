from __future__ import annotations

import json
from typing import Annotated, Literal

from pydantic import BaseModel, Field, StringConstraints, field_validator

from app.schemas.common import MAX_RESPONSE_METADATA_BYTES, SourceRef, bounded_json_object, bounded_token_usage

MAX_GENERATION_ID_LENGTH = 128
MAX_GENERATION_SHORT_TEXT_LENGTH = 128
MAX_GENERATION_TITLE_LENGTH = 255
MAX_TENDER_REQUIREMENT_LENGTH = 1000
MAX_TENDER_REQUIREMENTS = 50
MAX_REQUIREMENT_REFS = 50
MAX_SELECTED_KNOWLEDGE_REFS = 50
MAX_RETRIEVED_KNOWLEDGE_REFS = 20
MAX_RETRIEVED_KNOWLEDGE_CONTENT_LENGTH = 2400
MAX_REQUIREMENT_SOURCE_TEXT_LENGTH = 1200
MAX_REQUIREMENT_EXPECTED_RESPONSE_LENGTH = 1000
MAX_GENERATION_CALLBACK_URL_LENGTH = 2048
MAX_CHAPTER_ACTION_INSTRUCTION_LENGTH = 2000
MAX_CHAPTER_ACTION_PLAIN_TEXT_LENGTH = 30000
MAX_CHAPTER_ACTION_TIPTAP_JSON_BYTES = 256 * 1024
MAX_GENERATION_RESPONSE_SOURCE_REFS = 50
MAX_GENERATION_RESPONSE_NEEDS_HUMAN_INPUT = 20
MAX_GENERATION_RESPONSE_NOTE_LENGTH = 500
MAX_GENERATION_RESPONSE_TIPTAP_JSON_BYTES = 512 * 1024
MAX_GENERATION_RESPONSE_SELF_CHECK_BYTES = 128 * 1024
MAX_GENERATION_RESPONSE_METADATA_BYTES = MAX_RESPONSE_METADATA_BYTES

GenerationID = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_GENERATION_ID_LENGTH),
]
GenerationOptionalID = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_GENERATION_ID_LENGTH),
]
GenerationShortText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_GENERATION_SHORT_TEXT_LENGTH),
]
GenerationRequiredShortText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_GENERATION_SHORT_TEXT_LENGTH),
]
GenerationTitle = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_GENERATION_TITLE_LENGTH),
]
TenderRequirementText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_TENDER_REQUIREMENT_LENGTH),
]
TenderRequirementOptionalText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_REQUIREMENT_EXPECTED_RESPONSE_LENGTH),
]
GenerationReferenceContent = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_RETRIEVED_KNOWLEDGE_CONTENT_LENGTH),
]
GenerationRequirementSourceText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_REQUIREMENT_SOURCE_TEXT_LENGTH),
]
ChapterActionInstruction = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_CHAPTER_ACTION_INSTRUCTION_LENGTH),
]
ChapterActionPlainText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_CHAPTER_ACTION_PLAIN_TEXT_LENGTH),
]
GenerationCallbackURL = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_GENERATION_CALLBACK_URL_LENGTH),
]
GenerationResponseNote = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_GENERATION_RESPONSE_NOTE_LENGTH),
]
ChapterActionType = Literal["optimize", "expand", "shorten", "add_detail", "self_check"]
RequirementPriority = Literal["high", "medium", "low"]


class RetrievedKnowledgeRef(BaseModel):
    chunk_id: GenerationID
    document_id: GenerationID
    title: GenerationTitle
    section_path: GenerationShortText = ""
    content: GenerationReferenceContent = ""
    page_start: int | None = None
    page_end: int | None = None
    score: float = Field(default=0, ge=0, le=100, allow_inf_nan=False)


class TenderRequirementRef(BaseModel):
    id: GenerationID
    module: GenerationShortText = ""
    type: GenerationShortText = ""
    requirement: TenderRequirementText
    priority: RequirementPriority = "medium"
    mandatory: bool = False
    score: float | None = Field(default=None, ge=0, le=100, allow_inf_nan=False)
    expected_response: TenderRequirementOptionalText = ""
    status: GenerationShortText = ""
    source_text: GenerationRequirementSourceText = ""
    page_start: int | None = None
    page_end: int | None = None
    needs_review: bool = False


class ChapterGenerateRequest(BaseModel):
    task_id: GenerationOptionalID | None = None
    tenant_id: GenerationID
    bid_document_id: GenerationID
    bid_part_id: GenerationID
    chapter_id: GenerationID
    chapter_title: GenerationTitle
    tender_requirements: list[TenderRequirementText] = Field(default_factory=list, max_length=MAX_TENDER_REQUIREMENTS)
    requirement_refs: list[TenderRequirementRef] = Field(default_factory=list, max_length=MAX_REQUIREMENT_REFS)
    selected_knowledge_refs: list[GenerationID] = Field(default_factory=list, max_length=MAX_SELECTED_KNOWLEDGE_REFS)
    retrieved_knowledge_refs: list[RetrievedKnowledgeRef] = Field(default_factory=list, max_length=MAX_RETRIEVED_KNOWLEDGE_REFS)
    callback_url: GenerationCallbackURL | None = None
    model_hint: GenerationShortText | None = None


class ChapterActionRequest(ChapterGenerateRequest):
    action: ChapterActionType = "optimize"
    instruction: ChapterActionInstruction = ""
    current_plain_text: ChapterActionPlainText = ""
    current_tiptap_json: dict[str, object] = Field(default_factory=dict)

    @field_validator("current_tiptap_json")
    @classmethod
    def current_tiptap_json_must_be_bounded(cls, value: dict[str, object]) -> dict[str, object]:
        encoded = json.dumps(value, ensure_ascii=False, separators=(",", ":"))
        if len(encoded.encode("utf-8")) > MAX_CHAPTER_ACTION_TIPTAP_JSON_BYTES:
            raise ValueError("current_tiptap_json is too large")
        return value


class ChapterGenerateResponse(BaseModel):
    trace_id: GenerationRequiredShortText
    tiptap_json: dict[str, object]
    source_refs: list[SourceRef] = Field(default_factory=list, max_length=MAX_GENERATION_RESPONSE_SOURCE_REFS)
    self_check: dict[str, object]
    needs_human_input: list[GenerationResponseNote] = Field(
        default_factory=list,
        max_length=MAX_GENERATION_RESPONSE_NEEDS_HUMAN_INPUT,
    )
    model_metadata: dict[str, object]
    token_usage: dict[str, int]

    @field_validator("tiptap_json")
    @classmethod
    def tiptap_json_must_be_bounded(cls, value: dict[str, object]) -> dict[str, object]:
        return bounded_json_object(
            value,
            max_bytes=MAX_GENERATION_RESPONSE_TIPTAP_JSON_BYTES,
            label="tiptap_json",
        )

    @field_validator("self_check")
    @classmethod
    def self_check_must_be_bounded(cls, value: dict[str, object]) -> dict[str, object]:
        return bounded_json_object(
            value,
            max_bytes=MAX_GENERATION_RESPONSE_SELF_CHECK_BYTES,
            label="self_check",
        )

    @field_validator("model_metadata")
    @classmethod
    def model_metadata_must_be_bounded(cls, value: dict[str, object]) -> dict[str, object]:
        return bounded_json_object(
            value,
            max_bytes=MAX_GENERATION_RESPONSE_METADATA_BYTES,
            label="model_metadata",
        )

    @field_validator("token_usage")
    @classmethod
    def token_usage_must_be_bounded(cls, value: dict[str, int]) -> dict[str, int]:
        return bounded_token_usage(value)
