//  THIS CODE AND INFORMATION IS PROVIDED "AS IS" WITHOUT WARRANTY OF
//  ANY KIND, EITHER EXPRESSED OR IMPLIED, INCLUDING BUT NOT LIMITED TO
//  THE IMPLIED WARRANTIES OF MERCHANTABILITY AND/OR FITNESS FOR A
//  PARTICULAR PURPOSE.
//
//  Copyright  2001 - 2003  Microsoft Corporation.  All Rights Reserved.
//
//  FILE:    Features.cpp
//
//  PURPOSE:  Implementation wrapper class for PScript Driver Features and Options.
//

#include "precomp.h"
#include "resource.h"
#include "debug.h"
#include "globals.h"
#include "devmode.h"
#include "stringutils.h"
#include "helper.h"
#include "features.h"
#include "oemui.h"

// This indicates to Prefast that this is a usermode driver file.
__user_driver;


////////////////////////////////////////////////////////
//      Internal Defines and Macros
////////////////////////////////////////////////////////

#define INITIAL_ENUM_FEATURES_SIZE          1024
#define INITIAL_ENUM_OPTIONS_SIZE           64
#define INITIAL_FEATURE_DISPLAY_NAME_SIZE   64
#define INITIAL_OPTION_DISPLAY_NAME_SIZE    32
#define INITIAL_GET_OPTION_SIZE             64
#define INITIAL_GET_REASON_SIZE             1024

#define DRIVER_FEATURE_PREFIX               '%'
#define IS_DRIVER_FEATURE(f)                (DRIVER_FEATURE_PREFIX == (f)[0])

// Flags that the uDisplayNameID should be returned as
// MAKEINTRESOURCE() instead of loading the string resource.
#define RETURN_INT_RESOURCE     1

// Macros to test for conditions of KEYWORDMAP entry.
#define IS_MAPPING_INT_RESOURCE(p)  ((p)->dwFlags & RETURN_INT_RESOURCE)

// TAG the identifies feature OPTITEM data stuct.
#define FEATURE_OPTITEM_TAG     'FETR'


////////////////////////////////////////////////////////
//      Type Definitions
////////////////////////////////////////////////////////

// Struct used to identify OPTITEM as
// feature OPTITEM and to map back from
// an OPTITEM to the feature.
typedef struct _tagFeatureOptitemData
{} FEATUREOPTITEMDATA, *PFEATUREOPTITEMDATA;



////////////////////////////////////////////////////////
//      Internal Constants
////////////////////////////////////////////////////////

static KEYWORDMAP gkmFeatureMap[] =
{};
static const NUM_FEATURE_MAP    = (sizeof(gkmFeatureMap)/sizeof(gkmFeatureMap[0]));


static KEYWORDMAP gkmOptionMap[] =
{};
static const NUM_OPTION_MAP     = (sizeof(gkmOptionMap)/sizeof(gkmOptionMap[0]));




////////////////////////////////////////////
//
//  COptions Methods
//

//
//  Private Methods
//

// Initializes class data members.
void COptions::Init()
{}

void COptions::Clear()
{}

// Frees memory associated with Option Info array.
void COptions::FreeOptionInfo()
{}

// Will do init for features that need special handling.
HRESULT COptions::GetOptionsForSpecialFeatures(CUIHelper &Helper, POEMUIOBJ poemuiobj)
{}

// Do init for PS Level options.
HRESULT COptions::GetOptionsForOutputPSLevel(CUIHelper &Helper, POEMUIOBJ poemuiobj)
{}

HRESULT COptions::GetOptionSelectionString(CUIHelper &Helper, POEMUIOBJ poemuiobj, __out PSTR *ppszSel)
{}

// Gets current Options selection for LONG value.
HRESULT COptions::GetOptionSelectionLong(CUIHelper &Helper, POEMUIOBJ poemuiobj)
{}

// Gets current Options selection for SHORT value.
HRESULT COptions::GetOptionSelectionShort(CUIHelper &Helper, POEMUIOBJ poemuiobj)
{}

// Gets current option selection for feature.
HRESULT COptions::GetOptionSelectionIndex(CUIHelper &Helper, POEMUIOBJ poemuiobj)
{}



//
//  Public Methods
//

// Default constructor
COptions::COptions()
{}

// Destructor
COptions::~COptions()
{}

// Get the option list for a feature
HRESULT COptions::Acquire(__in HANDLE hHeap,
                          CUIHelper &Helper,
                          __in POEMUIOBJ poemuiobj,
                          __in PCSTR pszFeature)
{}

// Return nth options keyword.
PCSTR COptions::GetKeyword(WORD wIndex) const
{}

// Return nth options display name.
PCWSTR COptions::GetName(WORD wIndex) const
{}

// Find option with matching keyword string.
WORD COptions::FindOption(PCSTR pszOption, WORD wDefault) const
{}

// Initializes OptItem with options for a feature.
HRESULT COptions::InitOptItem(__in HANDLE hHeap, POPTITEM pOptItem)
{}

// Refresh option selection.
HRESULT COptions::RefreshSelection(CUIHelper &Helper, POEMUIOBJ poemuiobj)
{}



////////////////////////////////////////////
//
//  CFeatures Methods
//

//
//  Private Methods
//

// Initializes class
void CFeatures::Init()
{}

// Cleans up class and re-initialize it.
void CFeatures::Clear()
{}

// Free feature info
void CFeatures::FreeFeatureInfo()
{}

// Turns index for mode to modeless index, which
// is the real index to the feature.
WORD CFeatures::GetModelessIndex(WORD wIndex, DWORD dwMode) const
{}


//
//  Public Methods
//

// Default constructor
CFeatures::CFeatures()
{}

// Destructor
CFeatures::~CFeatures()
{}

// Gets Core Driver Features, if not already retrieved.
HRESULT CFeatures::Acquire(__in HANDLE hHeap,
                           CUIHelper &Helper,
                           __in POEMUIOBJ poemuiobj
                          )
{}

// Returns number of features contained in class instance.
WORD CFeatures::GetCount(DWORD dwMode) const
{}

// Returns nth feature's keyword
PCSTR CFeatures::GetKeyword(WORD wIndex, DWORD dwMode) const
{}

// Return nth feature's Display Name.
PCWSTR CFeatures::GetName(WORD wIndex, DWORD dwMode) const
{}

// Returns pointer to option class for nth feature.
COptions* CFeatures::GetOptions(WORD wIndex, DWORD dwMode) const
{}

// Formats OPTITEM for specied feature.
HRESULT CFeatures::InitOptItem(__in HANDLE hHeap, POPTITEM pOptItem, WORD wIndex, DWORD dwMode)
{}

//////////////////////////////////////////////////
//
//  Regular functions not part of class
//
//////////////////////////////////////////////////


// Maps feature keywords to display names for the features.
HRESULT DetermineFeatureDisplayName(__in HANDLE hHeap,
                                    CUIHelper &Helper,
                                    POEMUIOBJ poemuiobj,
                                    __in PCSTR pszKeyword,
                                    const PKEYWORDMAP pMapping,
                                    __deref_out_opt PWSTR *ppszDisplayName)
{}

// Maps option keywords to display names for the option.
HRESULT DetermineOptionDisplayName(__in HANDLE hHeap,
                                   CUIHelper &Helper,
                                   POEMUIOBJ poemuiobj,
                                   __in PCSTR pszFeature,
                                   __in PCSTR pszOption,
                                   const PKEYWORDMAP pMapping,
                                   __deref_out_opt PWSTR *ppszDisplayName)
{}

// Determines sticky mode for the feature.
HRESULT DetermineStickiness(CUIHelper &Helper,
                            POEMUIOBJ poemuiobj,
                            PCSTR pszKeyword,
                            const PKEYWORDMAP pMapping,
                            PDWORD pdwMode)
{}

// Find the mapping entry from the keyword.
PKEYWORDMAP FindKeywordMapping(PKEYWORDMAP pKeywordMap, WORD wMapSize, PCSTR pszKeyword)
{}

// Get display name from mapping entry.
HRESULT GetDisplayNameFromMapping(__in HANDLE hHeap, PKEYWORDMAP pMapping, __deref_out_opt PWSTR *ppszDisplayName)
{}

// Test if an OPTITEM is an OPTITEM for a feature.
// Macro for testing if OPTITEM is feature OPTITEM
BOOL IsFeatureOptitem(POPTITEM pOptItem)
{}



// Walks array of OPTITEMs saving each feature OPTITEM
// that has changed.
HRESULT SaveFeatureOptItems(__in HANDLE hHeap,
                            CUIHelper *pHelper,
                            POEMUIOBJ poemuiobj,
                            HWND hWnd,
                            POPTITEM pOptItem,
                            WORD wItems)
{}

// Allocates buffer, if needed, and calls IPrintCoreUI2::WhyConsrained
// to get reason for constraint.
HRESULT GetWhyConstrained(__in HANDLE hHeap,
                          CUIHelper *pHelper,
                          POEMUIOBJ poemuiobj,
                          __in PCSTR     pszFeature,
                          __in PCSTR     pszOption,
                          __inout PSTR   *ppmszReason,
                          __inout PDWORD pdwSize)
{}

// Creates multi-sz list of feature option pairs that have changed.
HRESULT GetChangedFeatureOptions(__in HANDLE           hHeap,
                                 POPTITEM              pOptItem,
                                 WORD                  wItems,
                                 __deref_out_opt PSTR  *ppmszPairs,
                                 __out PWORD           pwPairs,
                                 __out PDWORD          pdwSize)
{}

// Returns pointer to option keyword for a feature OPTITEM.
PSTR GetOptionKeywordFromOptItem(__in HANDLE hHeap, POPTITEM pOptItem)
{}

// Returns pointer to option display name for a feature OPTITEM.
PWSTR GetOptionDisplayNameFromOptItem(__in HANDLE hHeap, POPTITEM pOptItem)
{}

// Refreshes option selection for each feature OPTITEM
HRESULT RefreshOptItemSelection(CUIHelper *pHelper,
                                POEMUIOBJ poemuiobj,
                                POPTITEM pOptItems,
                                WORD wItems)
{}

// Creates array of conflict features.
HRESULT GetFirstConflictingFeature(__in HANDLE hHeap,
                                   CUIHelper *pHelper,
                                   POEMUIOBJ poemuiobj,
                                   POPTITEM pOptItem,
                                   WORD wItems,
                                   PCONFLICT pConflict)
{}




