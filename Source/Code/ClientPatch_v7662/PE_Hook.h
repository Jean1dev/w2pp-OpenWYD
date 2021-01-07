/*
*   Copyright (C) {2015}  {Victor Klafke, Charles TheHouse}
*
*   This program is free software: you can redistribute it and/or modify
*   it under the terms of the GNU General Public License as published by
*   the Free Software Foundation, either version 3 of the License, or
*   (at your option) any later version.
*
*   This program is distributed in the hope that it will be useful,
*   but WITHOUT ANY WARRANTY; without even the implied warranty of
*   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
*   GNU General Public License for more details.
*
*   You should have received a copy of the GNU General Public License
*   along with this program.  If not, see [http://www.gnu.org/licenses/].
*
*   Contact at: victor.klafke@ecomp.ufsm.br
*/

#ifndef __PE_HOOK_H__
#define __PE_HOOK_H__

#include <windows.h>
#include <stdio.h>

#define PTR2PTR(ptrInit) *(DWORD*)(*(DWORD*)ptrInit)

#define JMP_NEAR_SIZE 5
#define JNZ_NEAR_SIZE 6
#define JGE_NEAR_SIZE 6
#define JG_NEAR_SIZE 6
#define CALL_NEAR_SIZE 5
#define JE_NEAR_SIZE 3
#define JZ_NEAR_SIZE 4

#define DEF_STR(str, addr) strcpy((CHAR*)addr, (CONST CHAR*)str)

__inline void BuildIndirection(DWORD dwAddress, BYTE *opCode, DWORD opCodeSize, void *dwFuncAddr)
{
	DWORD
		dwOldProtect,
		dwInstructionSize = (opCodeSize + sizeof(DWORD)),
		dwDistance = (((DWORD)dwFuncAddr - dwAddress) - dwInstructionSize);

	BYTE
		*indirection = (BYTE*)malloc(opCodeSize + sizeof(DWORD));

	// Build OpCode Instruction
	fflush(stdin);
	memcpy((BYTE*)&indirection[0], opCode, opCodeSize);

	// Build Distance of Instruction
	memcpy((BYTE*)&indirection[opCodeSize], &dwDistance, sizeof(DWORD));

	// Write Instruction
	VirtualProtect((LPVOID)dwAddress, dwInstructionSize, PAGE_EXECUTE_READWRITE, &dwOldProtect);

	memcpy((DWORD*)dwAddress, indirection, dwInstructionSize);

	VirtualProtect((LPVOID)dwAddress, dwInstructionSize, dwOldProtect, &dwOldProtect);
}

__inline void FillWithNop(DWORD dwAddress, DWORD dwSize)
{
	DWORD
		dwOldProtect;

	BYTE
		*nop = (BYTE*)malloc(dwSize);

	VirtualProtect((LPVOID)dwAddress, dwSize, PAGE_EXECUTE_READWRITE, &dwOldProtect);

	for (DWORD i = 0; i < dwSize; i++)
		nop[i] = 0x90;

	memcpy((DWORD*)dwAddress, &nop[0], dwSize);

	VirtualProtect((LPVOID)dwAddress, dwSize, dwOldProtect, &dwOldProtect);
}

__inline void JMP_NEAR(DWORD dwAddress, void *dwFuncAddr)
{
	BYTE
		jmp = 0xE9;

	BuildIndirection(dwAddress, &jmp, 1, dwFuncAddr);
}

__inline void JMP_NEAR(DWORD dwAddress, void *dwFuncAddr, DWORD dwNopedSize)
{
	JMP_NEAR(dwAddress, dwFuncAddr);

	FillWithNop(dwAddress + JMP_NEAR_SIZE, dwNopedSize);
}

__inline void JGE_NEAR(DWORD dwAddress, void *dwFuncAddr)
{
	BYTE
		jge[2] = { 0x0F, 0x8D };

	BuildIndirection(dwAddress, jge, 2, dwFuncAddr);
}

__inline void JGE_NEAR(DWORD dwAddress, void *dwFuncAddr, DWORD dwNopedSize)
{
	JGE_NEAR(dwAddress, dwFuncAddr);

	FillWithNop(dwAddress + JGE_NEAR_SIZE, dwNopedSize);
}


__inline void JG_NEAR(DWORD dwAddress, void *dwFuncAddr)
{
	BYTE
		jg[2] = { 0x0F, 0x8F };

	BuildIndirection(dwAddress, jg, 2, dwFuncAddr);
}

__inline void JG_NEAR(DWORD dwAddress, void *dwFuncAddr, DWORD dwNopedSize)
{
	JGE_NEAR(dwAddress, dwFuncAddr);

	FillWithNop(dwAddress + JG_NEAR_SIZE, dwNopedSize);
}


__inline void JNZ_NEAR(DWORD dwAddress, void *dwFuncAddr)
{
	BYTE
		jnz[2] = { 0x0F, 0x85 };

	BuildIndirection(dwAddress, jnz, 2, dwFuncAddr);
}

__inline void JNZ_NEAR(DWORD dwAddress, void *dwFuncAddr, DWORD dwNopedSize)
{
	JNZ_NEAR(dwAddress, dwFuncAddr);

	FillWithNop(dwAddress + JNZ_NEAR_SIZE, dwNopedSize);
}

__inline void CALL_NEAR(DWORD dwAddress, void *dwFuncAddr)
{
	BYTE
		call = 0xE8;

	BuildIndirection(dwAddress, &call, 1, dwFuncAddr);
}

__inline void CALL_NEAR(DWORD dwAddress, void *dwFuncAddr, DWORD dwNopedSize)
{
	CALL_NEAR(dwAddress, dwFuncAddr);

	FillWithNop(dwAddress + CALL_NEAR_SIZE, dwNopedSize);
}

__inline void JE_NEAR(DWORD dwAddress, void *dwFuncAddr)
{
	BYTE
		je[2] = { 0x0F, 0x84 };

	BuildIndirection(dwAddress, je, 2, dwFuncAddr);
}

__inline void JE_NEAR(DWORD dwAddress, void *dwFuncAddr, DWORD dwNopedSize)
{
	JE_NEAR(dwAddress, dwFuncAddr);

	FillWithNop(dwAddress + JE_NEAR_SIZE, dwNopedSize);
}

__inline void JZ_NEAR(DWORD dwAddress, void *dwFuncAddr)
{
	BYTE
		jz[2] = { 0x0F, 0x84 };

	BuildIndirection(dwAddress, jz, 2, dwFuncAddr);
}

__inline void JZ_NEAR(DWORD dwAddress, void *dwFuncAddr, DWORD dwNopedSize)
{
	JZ_NEAR(dwAddress, dwFuncAddr);

	FillWithNop(dwAddress + JZ_NEAR_SIZE, dwNopedSize);
}

#endif // __PE_HOOK_H__